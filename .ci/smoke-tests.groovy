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
  }

  options {
    timeout(time: 3, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '100', artifactNumToKeepStr: '30', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
  }
  parameters {
    string(name: 'SMOKETEST_VERSIONS', defaultValue: 'latest', description: 'Run smoke tests using following APM versions')
  }
  stages {
    stage('Checkout') {
      options { skipDefaultCheckout() }
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", shallow: false)
      }
    }

    stage('Smoke Tests') {
      options { skipDefaultCheckout() }
      environment {
        SSH_KEY = "./id_rsa_terraform"
        TF_VAR_private_key = "./id_rsa_terraform"
        TF_VAR_public_key = "./id_rsa_terraform.pub"
        // cloud tags
        TF_VAR_BUILD_ID = "${env.BUILD_ID}"
        TF_VAR_ENVIRONMENT= 'ci'
        TF_VAR_BRANCH = "${env.BRANCH_NAME.toLowerCase().replaceAll('[^a-z0-9-]', '-')}"
        TF_VAR_REPO = "${REPO}"
      }
      steps {
        dir ("${BASE_DIR}") {
          withGoEnv() {
            withTestClusterEnv {
              sh(label: 'Run smoke tests', script: 'make smoketest')
            }
          }
        }
      }
    }
  }
}

def withTestClusterEnv(Closure body) {  
  withAWSEnv(secret: "${AWS_ACCOUNT_SECRET}", version: "2.7.6") {
    withTerraformEnv(version: "${TERRAFORM_VERSION}") {
      withSecretVault(secret: "${EC_KEY_SECRET}", data: ['apiKey': 'EC_API_KEY'] ) {
        body()
      }
    }
  }
}
