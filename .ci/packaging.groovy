#!/usr/bin/env groovy
@Library('apm@test/releaseManager') _

pipeline {
  agent none
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    SLACK_CHANNEL = 'UJ2J1AZV2'
    NOTIFY_TO = 'victor.martinez+apm-server@elastic.co'
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    JOB_GCS_CREDENTIALS = 'apm-ci-gcs-plugin'
    DOCKER_SECRET = 'secret/apm-team/ci/docker-registry/prod'
    DOCKER_REGISTRY = 'docker.elastic.co'
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
    // disable upstream trigger on a PR basis
    upstream("apm-server/apm-server-mbp/${ env.JOB_BASE_NAME.startsWith('PR-') ? 'none' : env.JOB_BASE_NAME }")
  }
  stages {
    stage('Filter build') {
      agent { label 'ubuntu-18 && immutable' }
      options { skipDefaultCheckout() }
      when {
        beforeAgent true
        anyOf {
          triggeredBy cause: "IssueCommentCause"
          expression {
            // TODO: test purposes
            return true
            def ret = isUserTrigger() || isUpstreamTrigger()
            if(!ret){
              currentBuild.result = 'NOT_BUILT'
              currentBuild.description = "The build has been skipped"
              currentBuild.displayName = "#${BUILD_NUMBER}-(Skipped)"
              echo("the build has been skipped due the trigger is a branch scan and the allowed ones are manual, GitHub comment, and upstream job")
            }
            return ret
          }
        }
      }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
      }
      stages {
        stage('Checkout') {
          options { skipDefaultCheckout() }
          steps {
            pipelineManager([ cancelPreviousRunningBuilds: [ when: 'PR' ] ])
            deleteDir()
            gitCheckout(basedir: "${BASE_DIR}", githubNotifyFirstTimeContributor: false,
                        shallow: false, reference: "/var/lib/jenkins/.git-references/${REPO}.git")
            stash allowEmpty: true, name: 'source', useDefaultExcludes: false
            // set environment variables globally since they are used afterwards but GIT_BASE_COMMIT won't
            // be available until gitCheckout is executed.
            //TODO: test purposes
            //setEnvVar('URI_SUFFIX', "commits/${env.GIT_BASE_COMMIT}")
            setEnvVar('URI_SUFFIX', "commits/a1531adc7407ff667a898c14c037bd4ec89127ba")
            // JOB_GCS_BUCKET contains the bucket and some folders, let's build the folder structure
            setEnvVar('PATH_PREFIX', "${JOB_GCS_BUCKET.contains('/') ? JOB_GCS_BUCKET.substring(JOB_GCS_BUCKET.indexOf('/') + 1) + '/' + env.URI_SUFFIX : env.URI_SUFFIX}")
            // TODO: test purposes
            //setEnvVar('IS_BRANCH_AVAILABLE', isBranchUnifiedReleaseAvailable(env.BRANCH_NAME))
            setEnvVar('IS_BRANCH_AVAILABLE', true)
            dir("${BASE_DIR}"){
              setEnvVar('VERSION', sh(label: 'Get version', script: 'make get-version', returnStdout: true)?.trim())
            }
          }
        }
        stage('Package') {
          //TODO: test purposes
          when { expression { return false } }
          options { skipDefaultCheckout() }
          matrix {
            agent {
              label "${PLATFORM}"
            }
            axes {
              axis {
                name 'PLATFORM'
                values 'linux && immutable', 'arm'
              }
              axis {
                name 'TYPE'
                values 'snapshot', 'staging'
              }
            }
            stages {
              stage('Package') {
                options { skipDefaultCheckout() }
                environment {
                  PLATFORMS = "${isArm() ? 'linux/arm64' : ''}"
                  PACKAGES = "${isArm() ? 'docker' : ''}"
                }
                steps {
                  runIfNoMainAndNoStaging() {
                    runPackage(type: env.TYPE)
                  }
                }
              }
              stage('Publish') {
                options { skipDefaultCheckout() }
                steps {
                  runIfNoMainAndNoStaging() {
                    publishArtifacts(type: env.TYPE)
                  }
                }
              }
            }
          }
          post {
            failure {
              notifyStatus(subject: "[${env.REPO}@${env.BRANCH_NAME}] package failed.",
                           body: 'Contact the Productivity team [#observablt-robots] if you need further assistance.')
            }
          }
        }
        stage('DRA') {
          options { skipDefaultCheckout() }
          // The Unified Release process keeps moving branches as soon as a new
          // minor version is created, therefore old release branches won't be able
          // to use the release manager as their definition is removed.
          when {
            expression { return env.IS_BRANCH_AVAILABLE == "true" }
          }
          environment {
            DRA_OUTPUT = 'release-manager-report.out'
          }
          steps {
            runReleaseManager(type: 'snapshot', outputFile: env.DRA_OUTPUT)
            whenFalse(env.BRANCH_NAME.equals('main')) {
              runReleaseManager(type: 'staging', outputFile: env.DRA_OUTPUT)
            }
          }
          post {
            failure {
              notifyStatus(analyse: true,
                           file: "${BASE_DIR}/${env.DRA_OUTPUT}",
                           subject: "[${env.REPO}@${env.BRANCH_NAME}] DRA failed.",
                           body: 'Contact the Release Platform team [#platform-release].')
            }
          }
        }
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
  }
}

def runReleaseManager(def args = [:]) {
  deleteDir()
  unstash 'source'
  def bucketLocation = getBucketLocation(args.type)
  googleStorageDownload(bucketUri: "${bucketLocation}/*",
                        credentialsId: "${JOB_GCS_CREDENTIALS}",
                        localDirectory: "${BASE_DIR}/build/distributions",
                        pathPrefix: "${env.PATH_PREFIX}/${args.type}")
  dir("${BASE_DIR}") {
    dockerLogin(secret: env.DOCKER_SECRET, registry: env.DOCKER_REGISTRY)
    sh(label: 'prepare-release-manager-artifacts', script: ".ci/scripts/prepare-release-manager.sh")
    //TODO: for testing purposes
    withEnv(["BRANCH_NAME=main"]){
    releaseManager(project: 'apm-server',
                   version: env.VERSION,
                   type: args.type,
                   artifactsFolder: 'build/distributions',
                   outputFile: args.outputFile)
    }
  }
}

def runPackage(def args = [:]) {
  def type = args.type
  def makeGoal = 'release-manager-snapshot'
  if (type.equals('staging')) {
    makeGoal = 'release-manager-release'
  }
  deleteDir()
  unstash 'source'
  dir("${BASE_DIR}"){
    withMageEnv() {
      sh(label: 'make release-manager', script: "make ${makeGoal}")
    }
  }
}

def publishArtifacts(def args = [:]) {
  def bucketLocation = getBucketLocation(args.type)
  // Copy those files to another location with the sha commit to test them afterward.
  googleStorageUpload(bucket: "${bucketLocation}",
    credentialsId: "${JOB_GCS_CREDENTIALS}",
    pathPrefix: "${BASE_DIR}/build/distributions/",
    pattern: "${BASE_DIR}/build/distributions/**/*",
    sharedPublicly: true,
    showInline: true)
  // Copy the dependencies files if no ARM
  whenFalse(isArm()) {
    googleStorageUpload(bucket: "${bucketLocation}",
      credentialsId: "${JOB_GCS_CREDENTIALS}",
      pathPrefix: "${BASE_DIR}/build/",
      pattern: "${BASE_DIR}/build/dependencies.csv",
      sharedPublicly: true,
      showInline: true)
  }
}

def getBucketLocation(type) {
  return "gs://${JOB_GCS_BUCKET}/${URI_SUFFIX}/${type}"
}

def notifyStatus(def args = [:]) {
  def releaseManagerFile = args.get('file', '')
  def analyse = args.get('analyse', false)
  def subject = args.get('subject', '')
  def body = args.get('body', '')
  releaseManagerNotification(file: releaseManagerFile,
                             analyse: analyse,
                             slackChannel: "${env.SLACK_CHANNEL}",
                             slackColor: 'danger',
                             slackCredentialsId: 'jenkins-slack-integration-token',
                             to: "${env.NOTIFY_TO}",
                             subject: subject,
                             body: "Build: (<${env.RUN_DISPLAY_URL}|here>).\n ${body}")
}

def runIfNoMainAndNoStaging(Closure body) {
  // TODO: test purposes
  //if (env.BRANCH_NAME.equals('main') && env.TYPE == 'staging') {
  if (env.TYPE == 'staging') {
    echo 'INFO: staging artifacts for the main branch are not required.'
  } else {
    body()
  }
}
