#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable && obs11' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    AWS_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-aws-account-auth'
    GCP_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-account-auth'
    EC_KEY_SECRET = 'secret/observability-team/ci/elastic-cloud/observability-pro'
    TERRAFORM_VERSION = '1.1.9'
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
      }
    }

    stage('Benchmarks') {
      options { skipDefaultCheckout() }
      steps {
        dir ("${BASE_DIR}") {
          withGoEnv() {
            dir("testing/benchmark") {          
              withTestClusterEnv {
                withECKey {     
                  withEnv(['SSH_KEY=./id_rsa_terraform', 'TF_VAR_public_key=./id_rsa_terraform.pub', 'TF_VAR_private_key=./id_rsa_terraform']) {                    
                    sh(label: 'Build apmbench', script: 'make apmbench $SSH_KEY terraform.tfvars')
                    sh(label: 'Spin up benchmark environment', script: 'make init apply')
                    archiveArtifacts(allowEmptyArchive: true, artifacts: "**/*.tfstate")
                    sh(label: 'Run benchmarks', script: 'make run-benchmark index-benchmark-results')
                  }
                }
              }
            }
          }
        }      
      }
      post {
        always {
          dir("${BASE_DIR}/testing/benchmark") {
            archiveArtifacts(allowEmptyArchive: true, artifacts: "**/*.tfstate") 
            withECKey {  
              withEnv(['SSH_KEY=./id_rsa_terraform', 'TF_VAR_public_key=./id_rsa_terraform.pub', 'TF_VAR_private_key=./id_rsa_terraform']) {                            
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
  withGCPEnv(secret: "${GCP_ACCOUNT_SECRET}") {
    withAWSEnv(secret: "${AWS_ACCOUNT_SECRET}") {
      withTerraformEnv(version: "${TERRAFORM_VERSION}") {
        body()
      }
    }
  }
}

def withECKey(Closure body) {
  def vaultResponse = getVaultSecret(secret: "${EC_KEY_SECRET}")
  if (vaultResponse.errors) {
    error("withECKey: Unable to get credentials from the vault: ${props.errors.toString()}")
  }

  def ecKey = vaultResponse?.data?.apiKey
  if (!ecKey?.trim()) {
    error("withECKey: Unable to read the apiKey field")
  }

  withEnvMask(vars: [
    [var: "EC_API_KEY", password: ecKey]
  ]) {
    body()
  }
}
