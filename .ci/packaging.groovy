#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent none
  environment {
    REPO = 'apm-server'
    REPO_BUILD_TAG = "${env.REPO}/${env.BUILD_TAG}/packaging"
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    SLACK_CHANNEL = '#apm-server'
    NOTIFY_TO = 'build-apm+apm-server@elastic.co'
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    JOB_GCS_CREDENTIALS = 'apm-ci-gcs-plugin'
    DOCKER_SECRET = 'secret/observability-team/ci/docker-registry/prod'
    DOCKER_REGISTRY = 'docker.elastic.co'
    DRA_OUTPUT = 'release-manager.out'
    COMMIT = "${params?.COMMIT}"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    DOCKER_IMAGE = "${env.DOCKER_REGISTRY}/observability-ci/apm-server"

    // Signing
    JOB_SIGNING_CREDENTIALS = 'sign-artifacts-with-gpg-job'
    INFRA_SIGNING_BUCKET_NAME = 'internal-ci-artifacts'
    INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER = "${env.REPO_BUILD_TAG}/signed-artifacts"
    INFRA_SIGNING_BUCKET_ARTIFACTS_PATH = "gs://${env.INFRA_SIGNING_BUCKET_NAME}/${env.REPO_BUILD_TAG}"
    INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH = "gs://${env.INFRA_SIGNING_BUCKET_NAME}/${env.INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER}"

    // Publishing
    INTERNAL_CI_JOB_GCS_CREDENTIALS = 'internal-ci-gcs-plugin'
    PACKAGE_STORAGE_UPLOADER_CREDENTIALS = 'upload-package-to-package-storage'
    PACKAGE_STORAGE_UPLOADER_GCP_SERVICE_ACCOUNT = 'secret/gce/elastic-bekitzur/service-account/package-storage-uploader'
    PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH = "gs://elastic-bekitzur-package-storage-internal/queue-publishing/${env.REPO_BUILD_TAG}"
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
            smartGitCheckout()
            stash(allowEmpty: true, name: 'source', useDefaultExcludes: false)
            // set environment variables globally since they are used afterwards but GIT_BASE_COMMIT won't
            // be available until gitCheckout is executed.
            setEnvVar('URI_SUFFIX', "commits/${env.GIT_BASE_COMMIT}")
            // JOB_GCS_BUCKET contains the bucket and some folders, let's build the folder structure
            setEnvVar('PATH_PREFIX', "${JOB_GCS_BUCKET.contains('/') ? JOB_GCS_BUCKET.substring(JOB_GCS_BUCKET.indexOf('/') + 1) + '/' + env.URI_SUFFIX : env.URI_SUFFIX}")
            setEnvVar('IS_BRANCH_AVAILABLE', isBranchUnifiedReleaseAvailable(env.BRANCH_NAME))
            dir("${BASE_DIR}"){
              setEnvVar('VERSION', sh(label: 'Get version', script: 'make get-version', returnStdout: true)?.trim())
            }
          }
        }
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
              }
            }
          }
          post {
            failure {
              whenTrue(isBranch()) {
                notifyStatus(subject: "[${env.REPO}@${env.BRANCH_NAME}] package failed.",
                             body: 'Contact the Productivity team [#observablt-robots] if you need further assistance.')
              }
            }
          }
        }
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
              }
            }
          }
          post {
            failure {
              notifyStatus(subject: "[${env.REPO}@${env.BRANCH_NAME}] apmpackage failed")
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
    releaseManager(project: 'apm-server',
                   version: env.VERSION,
                   type: args.type,
                   artifactsFolder: 'build/distributions',
                   outputFile: args.outputFile)
  }
}

def runWithGo(Closure body) {
  deleteDir()
  unstash 'source'
  dir("${BASE_DIR}"){
    withGoEnv() {
      body()
    }
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
    }
  }
}

// runPackage builds the distribution packages: tarballs, RPMs, Docker images, etc.
// We run this on linux/amd64 and linux/arm64. On linux/amd64 we build all packages;
// on linux/arm64 we build only Docker images.
def runPackage(def args = [:]) {
  def type = args.type
  def makeGoal = isArm() ? 'package-docker' : 'package'
  makeGoal += type.equals('snapshot') ? '-snapshot' : ''
  runWithGo() {
    sh(label: "make ${makeGoal}", script: "make ${makeGoal}")
  }
}

def publishArtifactsDev() {
  def dockerImage = readFile(file: "${BASE_DIR}/build/docker/apm-server-${env.VERSION}-SNAPSHOT.txt").trim()
  def bucketLocation = "gs://${JOB_GCS_BUCKET}/pull-requests/pr-${env.CHANGE_ID}"
  if (isPR()) {
    bucketLocation = "gs://${JOB_GCS_BUCKET}/snapshots"
  }
  uploadArtifacts(bucketLocation: bucketLocation)
  uploadArtifacts(bucketLocation: "gs://${JOB_GCS_BUCKET}/${URI_SUFFIX}")

  dockerLogin(secret: env.DOCKER_SECRET, registry: env.DOCKER_REGISTRY)
  sh(label: 'Tag Docker image', script: "docker tag ${dockerImage} ${env.DOCKER_IMAGE}:${env.GIT_BASE_COMMIT}")
  retryWithSleep(retries: 3, seconds: 5, backoff: true) {
    sh(label: 'Push Docker image', script: "docker push ${env.DOCKER_IMAGE}:${env.GIT_BASE_COMMIT}")
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
      pattern: "${BASE_DIR}/build/dependencies*.csv",
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
  if (env.BRANCH_NAME.equals('main') && env.TYPE == 'staging') {
    echo 'INFO: staging artifacts for the main branch are not required.'
  } else {
    body()
  }
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

def packageStoragePublish(builtPackagesPath, glob) {
  def unpublished = signUnpublishedArtifactsWithElastic(builtPackagesPath, glob)
  if (unpublished.isEmpty()) {
    echo 'All packages have been published already'
    return
  }
  uploadUnpublishedToPackageStorage(builtPackagesPath, unpublished)
}

def signUnpublishedArtifactsWithElastic(builtPackagesPath, glob) {
  def unpublished = []
  dir(builtPackagesPath) {
    findFiles(glob: glob)?.collect{ it.name }?.sort()?.each {
      def packageZip = it
      if (isAlreadyPublished(packageZip)) {
        return
      }

      unpublished.add(packageZip)
      googleStorageUpload(bucket: env.INFRA_SIGNING_BUCKET_ARTIFACTS_PATH,
        credentialsId: env.INTERNAL_CI_JOB_GCS_CREDENTIALS,
        pattern: packageZip,
        sharedPublicly: false,
        showInline: true)
    }
  }

  if (unpublished.isEmpty()) {
    return unpublished
  }

  withCredentials([string(credentialsId: env.JOB_SIGNING_CREDENTIALS, variable: 'TOKEN')]) {
    triggerRemoteJob(auth: CredentialsAuth(credentials: 'local-readonly-api-token'),
      job: 'https://internal-ci.elastic.co/job/elastic+unified-release+master+sign-artifacts-with-gpg',
      token: TOKEN,
      parameters: [
        gcs_input_path: env.INFRA_SIGNING_BUCKET_ARTIFACTS_PATH,
      ],
      useCrumbCache: false,
      useJobInfoCache: false)
  }
  googleStorageDownload(bucketUri: "${env.INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH}/*",
    credentialsId: env.INTERNAL_CI_JOB_GCS_CREDENTIALS,
    localDirectory: builtPackagesPath + '/',
    pathPrefix: "${env.INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER}")
    sh(label: 'Rename .asc to .sig', script: 'for f in ' + builtPackagesPath + '/*.asc; do mv "$f" "${f%.asc}.sig"; done')
  archiveArtifacts(allowEmptyArchive: false, artifacts: "${builtPackagesPath}/*.sig")
  return unpublished
}

def uploadUnpublishedToPackageStorage(builtPackagesPath, packageZips) {
  dir(builtPackagesPath) {
    withGCPEnv(secret: env.PACKAGE_STORAGE_UPLOADER_GCP_SERVICE_ACCOUNT) {
      withCredentials([string(credentialsId: env.PACKAGE_STORAGE_UPLOADER_CREDENTIALS, variable: 'TOKEN')]) {
        packageZips.each {
          def packageZip = it

          sh(label: 'Upload package .zip file', script: "gsutil cp ${packageZip} ${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/")
          sh(label: 'Upload package .sig file', script: "gsutil cp ${packageZip}.sig ${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/")

          triggerRemoteJob(auth: CredentialsAuth(credentials: 'local-readonly-api-token'),
            job: 'https://internal-ci.elastic.co/job/package_storage/job/publishing-job-remote',
            token: TOKEN,
            parameters: [
              gs_package_build_zip_path: "${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip}",
              gs_package_signature_path: "${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip}.sig",
              dry_run: false,
            ],
            useCrumbCache: true,
            useJobInfoCache: true)
        }
      }
    }
  }
}

def isAlreadyPublished(packageZip) {
  def responseCode = httpRequest(method: "HEAD",
    url: "https://package-storage.elastic.co/artifacts/packages/${packageZip}",
    response_code_only: true)
  return responseCode == 200
}
