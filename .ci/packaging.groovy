#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent none
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    SLACK_CHANNEL = '#apm-server'
    NOTIFY_TO = 'build-apm+apm-server@elastic.co'
    DOCKER_SECRET = 'secret/observability-team/ci/docker-registry/prod'
    DOCKER_REGISTRY = 'docker.elastic.co'
    COMMIT = "${params?.COMMIT}"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    DOCKER_IMAGE = "${env.DOCKER_REGISTRY}/observability-ci/apm-server"
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
  parameters {
    string(name: 'COMMIT', defaultValue: '', description: 'The Git commit to be used (empty will checkout the latest commit)')
  }
  stages {
    stage('Filter build') {
      agent { label 'ubuntu-18 && immutable' }
      options { skipDefaultCheckout() }
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
            smartGitCheckout()
            stash(allowEmpty: true, name: 'source', useDefaultExcludes: false)
            dir("${BASE_DIR}"){
              setEnvVar('VERSION', sh(label: 'Get version', script: 'make get-version', returnStdout: true)?.trim())
            }
          }
        }
<<<<<<< HEAD
        stage('Package') {
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
                  withGithubNotify(context: "Package-${TYPE}-${PLATFORM}") {
                    runIfNoMainAndNoStaging() {
                      runPackage(type: env.TYPE)
                    }
                  }
                }
              }
              stage('Publish') {
                options { skipDefaultCheckout() }
                steps {
                  withGithubNotify(context: "Publish-${TYPE}-${PLATFORM}") {
                    runIfNoMainAndNoStaging() {
                      publishArtifacts()
                    }
                  }
                }
=======
        stage('apmpackage') {
          options { skipDefaultCheckout() }
          when {
            allOf {
              // The apmpackage stage gets triggered as described in https://github.com/elastic/apm-server/issues/6970
              changeset pattern: '(internal/version/.*|apmpackage/.*)', comparator: 'REGEXP'
              not { changeRequest() }
            }
          }
          steps {
            withGithubNotify(context: 'apmpackage') {
              runWithGo() {
                // Build a preview package which includes the Git commit timestamp, and upload it to package storage.
                // Note, we intentionally do not sign or upload the "release" package, as it does not include a timestamp,
                // and will break package storage's immutability requirement.
                sh(label: 'make build-package-snapshot', script: 'make build-package-snapshot')
                packageStoragePublish('build/packages', 'apm-*-preview-*.zip')
                archiveArtifacts(allowEmptyArchive: false, artifacts: 'build/packages/*.zip')
>>>>>>> 56cc6e896 (jenkins: remove DRA (#11413))
              }
            }
          }
          post {
            failure {
<<<<<<< HEAD
              notifyStatus(subject: "[${env.REPO}@${env.BRANCH_NAME}] package failed.",
                           body: 'Contact the Productivity team [#observablt-robots] if you need further assistance.')
            }
          }
        }
        stage('DRA Snapshot') {
          options { skipDefaultCheckout() }
          // The Unified Release process keeps moving branches as soon as a new
          // minor version is created, therefore old release branches won't be able
          // to use the release manager as their definition is removed.
          when {
            expression { return env.IS_BRANCH_AVAILABLE == "true" }
          }
          steps {
            runReleaseManager(type: 'snapshot', outputFile: env.DRA_OUTPUT)
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
        stage('DRA Staging') {
          options { skipDefaultCheckout() }
          when {
            allOf {
              // The Unified Release process keeps moving branches as soon as a new
              // minor version is created, therefore old release branches won't be able
              // to use the release manager as their definition is removed.
              expression { return env.IS_BRANCH_AVAILABLE == "true" }
              not { branch 'main' }
            }
          }
          steps {
            runReleaseManager(type: 'staging', outputFile: env.DRA_OUTPUT)
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
=======
              notifyStatus(subject: "[${env.REPO}@${env.BRANCH_NAME}] apmpackage failed")
            }
          }
        }
>>>>>>> 56cc6e896 (jenkins: remove DRA (#11413))
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
  }
}

<<<<<<< HEAD
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
    sh(label: "prepare-release-manager-artifacts ${args.type}", script: ".ci/scripts/prepare-release-manager.sh ${args.type}")
    releaseManager(project: 'apm-server',
                   version: env.VERSION,
                   type: args.type,
                   artifactsFolder: 'build/distributions',
                   outputFile: args.outputFile)
  }
}

def publishArtifacts() {
  if(env.IS_BRANCH_AVAILABLE == "true") {
    publishArtifactsDRA(type: env.TYPE)
  } else {
    if (env.TYPE == "snapshot" && !isArm()) {
      publishArtifactsDev()
    } else {
      echo "publishArtifacts: type is not required to be published for this particular branch/PR"
=======
def runWithGo(Closure body) {
  deleteDir()
  unstash 'source'
  dir("${BASE_DIR}"){
    withGoEnv() {
      body()
>>>>>>> 56cc6e896 (jenkins: remove DRA (#11413))
    }
  }
}

<<<<<<< HEAD
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

def publishArtifactsDev() {
  def bucketLocation = "gs://${JOB_GCS_BUCKET}/pull-requests/pr-${env.CHANGE_ID}"
  if (isPR()) {
    bucketLocation = "gs://${JOB_GCS_BUCKET}/snapshots"
  }
  uploadArtifacts(bucketLocation: bucketLocation)
  uploadArtifacts(bucketLocation: "gs://${JOB_GCS_BUCKET}/${URI_SUFFIX}")

  dockerLogin(secret: env.DOCKER_SECRET, registry: env.DOCKER_REGISTRY)
  dir("${BASE_DIR}"){
    sh(label: 'Push', script: "./.ci/scripts/push-docker.sh ${env.GIT_BASE_COMMIT} ${env.DOCKER_IMAGE}")
  }
}

def publishArtifactsDRA(def args = [:]) {
  uploadArtifacts(bucketLocation: getBucketLocation(args.type))
}

def uploadArtifacts(def args = [:]) {
  // Copy those files to another location with the sha commit to test them afterward.
  googleStorageUpload(bucket: "${args.bucketLocation}",
    credentialsId: "${JOB_GCS_CREDENTIALS}",
    pathPrefix: "${BASE_DIR}/build/distributions/",
    pattern: "${BASE_DIR}/build/distributions/**/*",
    sharedPublicly: true,
    showInline: true)
  // Copy the dependencies files if no ARM
  whenFalse(isArm()) {
    googleStorageUpload(bucket: "${args.bucketLocation}",
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

=======
>>>>>>> 56cc6e896 (jenkins: remove DRA (#11413))
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

def smartGitCheckout() {
  // Checkout the given commit
  if (env.COMMIT?.trim()) {
    gitCheckout(basedir: "${BASE_DIR}",
                branch: "${env.COMMIT}",
                credentialsId: "${JOB_GIT_CREDENTIALS}",
                repo: "https://github.com/elastic/${REPO}.git")
  } else {
    gitCheckout(basedir: "${BASE_DIR}",
                githubNotifyFirstTimeContributor: false,
                shallow: false)
  }
}
