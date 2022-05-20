#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    AWS_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-aws-account-auth'
    GCP_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-account-auth'
    EC_KEY_SECRET = 'secret/observability-team/ci/elastic-cloud/observability-team'
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

  stages {
    stage('Checkout') {
      options { skipDefaultCheckout() }
      steps {        
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", shallow: false)
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false, excludes: '.git'        
      }
    }

    stage('Benchmarks') {
      steps {
        dir("${BASE_DIR}/testing/benchmark") {
          sh(label: 'Build apmbench', script: 'apmbench')
          withTestClusterEnv {
            withECKey {
              withGoEnv {
                sh(label: 'Spin up benchmark environment', script: 'make init apply')
                archiveArtifacts(allowEmptyArchive: true, artifacts: "**/*.tfstate")
                sh(label: 'ls', script: 'ls -lah')
                sh(label: 'Run benchmarks', script: 'make run-benchmark index-benchmark-results')
              }
            }
          }
        }        
      }
      post {
        always {
          dir("${BASE_DIR}/testing/benchmark") {      
            archiveArtifacts(allowEmptyArchive: true, artifacts: "**/*.tfstate")      
            withTestClusterEnv {
              sh(label: 'Tear down benchmark environment', script: 'make destroy')
            }            
          }
        }
      }
    }
  }
}

def withTestClusterEnv(Closure body) {
  withGCPEnv(secret: "${GCP_ACCOUNT_SECRET}") {
    withAWSEnv(secret: "${AWS_ACCOUNT_SECRET}") {
      withTerraformEnv(version: "${TERRAFORM_VERSION}") {
        body()
      }
    }
  }
}

def withECKey(Closure body) {
  def ecKey = getVaultSecret("${EC_KEY_SECRET}")?.data.apiKey
  withEnvMask(vars: [
    [var: "EC_API_KEY", password: ecKey]
  ]) {
    body()
  }
}
