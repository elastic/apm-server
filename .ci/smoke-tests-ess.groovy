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
    SLACK_CHANNEL = "#apm-server"
    SMOKETEST_VERSIONS = "${params.SMOKETEST_VERSIONS}"
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
    string(name: 'SMOKETEST_VERSIONS', defaultValue: '7.17,latest', description: 'Run smoke tests using following APM versions')
  }
  stages {
    stage('Checkout') {
      options { skipDefaultCheckout() }
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", shallow: false)
        stash(allowEmpty: true, name: 'source', useDefaultExcludes: false)
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
          withTestClusterEnv {
            script {
              def smokeTests = sh(returnStdout: true, script: 'make smoketest/discover').trim().split('\r?\n')
              log(level: 'INFO', text: "make smoketest/discover: '${smokeTests}'")
              def smokeTestJobs = [:]
              for (smokeTest in smokeTests) {
                // get the title for the stage based on the basename of the smoke test full path to be run
                def title = sh(script: "basename ${smokeTest}", returnStdout:true).trim()
                env.SMOKETEST_VERSIONS.trim().split(',').each { version ->
                  smokeTestJobs["${version}-${title}"] = runSmokeTestWithVersion(smokeTest: smokeTest, version: version, title: title)
                }
              }
              parallel smokeTestJobs
            }
          }
        }
      }
      post {
        always {
          dir("${BASE_DIR}") {
            withTestClusterEnv {
              sh(label: 'Teardown smoke tests infra', script: 'make smoketest/all/cleanup')
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

def runSmokeTestWithVersion(Map args = [:]) {
  def testDir = args.smokeTest
  def title = args.get('title', testDir)
  def version = args.version
  return {
    withNode(labels: 'linux && immutable', forceWorker: true) {
      deleteDir()
      unstash 'source'
      dir("${BASE_DIR}") {
        withTestClusterEnv {
          sh(label: "Run smoke tests ${testDir} for ${version}", script: "make smoketest/run-version TEST_DIR=${testDir} SMOKETEST_VERSION=${version}")
        }
      }
    }
  }
}

def withTestClusterEnv(Closure body) {
  withAWSEnv(secret: "${AWS_ACCOUNT_SECRET}", version: "2.7.6") {
    withTerraformEnv(version: "${TERRAFORM_VERSION}", forceInstallation: true) {
      withSecretVault(secret: "${EC_KEY_SECRET}", data: ['apiKey': 'EC_API_KEY'] ) {
        withGoEnv() {
          body()
        }
      }
    }
  }
}
