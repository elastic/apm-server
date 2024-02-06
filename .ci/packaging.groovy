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
    DOCKER_SECRET = 'secret/observability-team/ci/docker-registry/prod'
    DOCKER_REGISTRY = 'docker.elastic.co'
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
      agent { label 'ubuntu-22 && immutable' }
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
        stage('apmpackage') {
          options { skipDefaultCheckout() }
          when {
            anyOf {
              allOf {
                // The apmpackage stage gets triggered as described in https://github.com/elastic/apm-server/issues/6970
                changeset pattern: '(internal/version/.*|apmpackage/.*)', comparator: 'REGEXP'
                not { changeRequest() }
              }
              // support for manually triggered in the UI
              expression {
                return = isUserTrigger()
              }
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
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
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
