#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    AWS_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-aws-account-auth'
    EC_KEY_SECRET = 'secret/observability-team/ci/elastic-cloud/observability-pro'
    TERRAFORM_VERSION = '1.2.3'
    SLACK_CHANNEL = "#apm-server"
    // cloud tags
    TF_VAR_BUILD_ID = "${env.BUILD_ID}"
    TF_VAR_ENVIRONMENT= 'ci'
    TF_VAR_BRANCH = "${env.BRANCH_NAME.toLowerCase().replaceAll('[^a-z0-9-]', '-')}"
    TF_VAR_REPO = "${REPO}"
    TF_VAR_CREATED_DATE = "${new Date().getTime()}"
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '100', artifactNumToKeepStr: '30', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
  }
  stages {
    stage('Checkout') {
      options { skipDefaultCheckout() }
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", shallow: false)
        setEnvVar('GO_VERSION', readFile(file: "${BASE_DIR}/.go-version").trim())
        dir("${BASE_DIR}"){
          setEnvVar('VERSION', sh(label: 'Get version', script: 'make get-version', returnStdout: true)?.trim())
        }
      }
    }
    stage('Smoke Tests') {
      options { skipDefaultCheckout() }
      steps {
        dir("${BASE_DIR}/testing/smoke/supported-os") {
          withTestClusterEnv() {
            sh(label: "Run smoke tests", script: './test.sh ${VERSION}-SNAPSHOT')
          }
        }
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(slackComment: true)
    }
  }
}

def withTestClusterEnv(Closure body) {
  // Run withGoEnv earlier since the HOME is set on the fly
  // otherwise it might fail with error configuring Terraform AWS Provider: failed to get shared config profile
  withGoEnv() {
    withAWSEnv(secret: "${AWS_ACCOUNT_SECRET}", version: "2.7.6") {
      withTerraformEnv(version: "${TERRAFORM_VERSION}", forceInstallation: true) {
        withSecretVault(secret: "${EC_KEY_SECRET}", data: ['apiKey': 'EC_API_KEY'] ) {
          body()
        }
      }
    }
  }
}
