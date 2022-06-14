#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    BRANCH_NAME_LOWER_CASE = "${env.BRANCH_NAME.toLowerCase().replaceAll('[^a-z0-9-]', '-')}"
    AWS_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-aws-account-auth'    
    EC_KEY_SECRET = 'secret/observability-team/ci/elastic-cloud/observability-pro'
    BENCHMARK_ES_SECRET = 'secret/apm-team/ci/benchmark-cloud'
    TERRAFORM_VERSION = '1.1.9'
    
    CREATED_DATE = "${new Date().getTime()}"
    JOB_GCS_BUCKET_STASH = 'apm-ci-temp'
    JOB_GCS_CREDENTIALS = 'apm-ci-gcs-plugin'
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

  stages {
    stage('Checkout') {
      options { skipDefaultCheckout() }
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", shallow: false)
      }
    }

    stage('Benchmarks') {
      options { skipDefaultCheckout() }
      environment {
        // cloud tags
        TF_VAR_BUILD_ID = "${env.BUILD_ID}"
        TF_VAR_BRANCH_NAME = "${env.BRANCH_NAME}"
        TF_VAR_ENVIRONMENT= 'ci'
        TF_VAR_BRANCH = "${BRANCH_NAME_LOWER_CASE}"
        TF_VAR_REPO = "${REPO}"
        //profiles
        ESS_SYSTEM_PROFILE = '1GBx1zone'
        BENCHMARK_PROFILE = '64agents'
        
        GOBENCH_TAGS = "branch=${BRANCH_NAME_LOWER_CASE},benchmark_profile=${BENCHMARK_PROFILE},system_profile=${ESS_SYSTEM_PROFILE}"
      }
      steps {
        dir ("${BASE_DIR}") {
          withGoEnv() {
            dir("testing/benchmark") {
              withTestClusterEnv {
                sh(label: 'Build apmbench', script: 'make apmbench $SSH_KEY terraform.tfvars')                
                sh(label: 'Spin up benchmark environment', script: '$(make docker-override-committed-version) && make init apply')
                withESBenchmarkEnv {
                  sh(label: 'Run benchmarks', script: 'make run-benchmark index-benchmark-results')
                }
              }
            }
          }
        }
      }
      post {
        always {
          dir("${BASE_DIR}") {
            withGoEnv() {
              dir("testing/benchmark") {
                stashV2(name: 'benchmark_tfstate', bucket: "${JOB_GCS_BUCKET_STASH}", credentialsId: "${JOB_GCS_CREDENTIALS}")
                withTestClusterEnv {                  
                  sh(label: 'Tear down benchmark environment', script: 'make destroy')
                }
              }  
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

def withESBenchmarkEnv(Closure body) {
  withSecretVault(
      secret: "${BENCHMARK_ES_SECRET}", 
      data: ['user': 'GOBENCH_USERNAME', 
            'url': 'GOBENCH_HOST', 
            'password': 'GOBENCH_PASSWORD'] ) {
    body()
  }
}
