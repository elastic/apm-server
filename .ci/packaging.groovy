#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent none
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    SLACK_CHANNEL = '#apm-server'
    NOTIFY_TO = 'build-apm+apm-server@elastic.co'
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    JOB_GCS_CREDENTIALS = 'apm-ci-gcs-plugin'
    DOCKER_SECRET = 'secret/apm-team/ci/docker-registry/prod'
    DOCKER_REGISTRY = 'docker.elastic.co'
    DOCKER_IMAGE = "${env.DOCKER_REGISTRY}/observability-ci/apm-server"
    ONLY_DOCS = "false"
  }
  options {
    timeout(time: 2, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '100', artifactNumToKeepStr: '30', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
    quietPeriod(10)
  }
  triggers {
    issueCommentTrigger('(?i)^\\/package$')
    // disable upstream trigger on a PR basis
    upstream("apm-server/apm-server-mbp/${ env.JOB_BASE_NAME.startsWith('PR-') ? 'none' : env.JOB_BASE_NAME }")
  }
  stages {
    stage('Filter build') {
      agent { label 'ubuntu-18 && immutable' }
      when {
        beforeAgent true
        anyOf {
          triggeredBy cause: "IssueCommentCause"
          expression {
            def ret = isUserTrigger() || isUpstreamTrigger()
            if(!ret){
              currentBuild.result = 'NOT_BUILT'
              currentBuild.description = "The build has been skipped"
              currentBuild.displayName = "#${BUILD_NUMBER}-(Skipped)"
              echo("the build has been skipped due the trigger is a branch scan and the allow ones are manual, GitHub comment, and upstream job")
            }
            return ret
          }
        }
      }
      stage('Checkout') {
        environment {
          PATH = "${env.PATH}:${env.WORKSPACE}/bin"
          HOME = "${env.WORKSPACE}"
        }
        options { skipDefaultCheckout() }
        steps {
          pipelineManager([ cancelPreviousRunningBuilds: [ when: 'PR' ] ])
          deleteDir()
          gitCheckout(basedir: "${BASE_DIR}", githubNotifyFirstTimeContributor: true,
                      shallow: false, reference: "/var/lib/jenkins/.git-references/${REPO}.git")
          stash allowEmpty: true, name: 'source', useDefaultExcludes: false
          dir("${BASE_DIR}"){
            setEnvVar('ONLY_DOCS', isGitRegionMatch(patterns: [ '.*\\.asciidoc' ], comparator: 'regexp', shouldMatchAll: true))
          }
        }
      }
      stage('Package') {
        when {
          beforeAgent true
          allOf {
            expression { return env.ONLY_DOCS == "false" }
            anyOf {
              branch 'main'
              branch pattern: '\\d+\\.\\d+', comparator: 'REGEXP'
              expression { return env.GITHUB_COMMENT?.contains('/package')}
            }
          }
        }
        options { skipDefaultCheckout() }
        environment {
          PATH = "${env.PATH}:${env.WORKSPACE}/bin"
          HOME = "${env.WORKSPACE}"
          SNAPSHOT = "true"
        }
        matrix {
          agent {
            label "${PLATFORM}"
          }
          axes {
            axis {
              name 'PLATFORM'
              values 'linux && immutable', 'arm'
            }
          }
          stages {
            stage('Package') {
              environment {
                PLATFORMS = "${isArm() ? 'linux/arm64' : 'linux/amd64'}"
                NEW_TAG =  "${isArm() ? env.GIT_BASE_COMMIT + '-arm' : env.GIT_BASE_COMMIT}"
              }
              steps {
                withGithubNotify(context: "Package-${PLATFORM}") {
                  deleteDir()
                  unstash 'source'
                  dir("${BASE_DIR}"){
                    withMageEnv(){
                      sh(label: 'Build packages', script: './.ci/scripts/package.sh')
                      dockerLogin(secret: env.DOCKER_SECRET, registry: env.DOCKER_REGISTRY)
                      sh(label: 'Package & Push', script: "./.ci/scripts/package-docker-snapshot.sh ${NEW_TAG} ${env.DOCKER_IMAGE}")
                    }
                  }
                }
              }
            }
            stage('DRA') {
              when {
                beforeAgent true
                anyOf {
                  branch 'main'
                  branch pattern: '\\d+\\.\\d+', comparator: 'REGEXP'
                }
              }
              steps {
                echo 'TBD'
              }
            }
          }
        }
        post {
          failure {
            notifyStatus(slackStatus: 'danger', subject: "[${env.REPO}] DRA failed", body: "Build: (<${env.RUN_DISPLAY_URL}|here>)")
          }
        }
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult()
    }
  }
}

def notifyStatus(def args = [:]) {
  releaseNotification(slackChannel: "${env.SLACK_CHANNEL}",
                      slackColor: args.slackStatus,
                      slackCredentialsId: 'jenkins-slack-integration-token',
                      to: "${env.NOTIFY_TO}",
                      subject: args.subject,
                      body: args.body)
}
