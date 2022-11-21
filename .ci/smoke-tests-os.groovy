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
    CREATED_DATE = "${new Date().getTime()}"
    SLACK_CHANNEL = "#observablt-bots"
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
      }
    }
    stage('Smoke Tests') {
      options { skipDefaultCheckout() }
      steps {
        withAWSEnv(secret: "${AWS_ACCOUNT_SECRET}", version: "2.7.6") {
          withTerraformEnv(version: "${TERRAFORM_VERSION}", forceInstallation: true) {
            withSecretVault(secret: "${EC_KEY_SECRET}", data: ['apiKey': 'EC_API_KEY'] ) {
              dir("${BASE_DIR}") {
                withGoEnv() {
                  sh(label: "Run smoke tests", script: 'testing/smoke/supported-os/test.sh')
                }
              }
            }
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
