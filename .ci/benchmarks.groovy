#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    AWS_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-aws-account-auth'
    EC_KEY_SECRET = 'secret/observability-team/ci/elastic-cloud/observability-pro'
    BENCHMARK_ES_SECRET = 'secret/observability-team/ci/benchmark-cloud'
    BENCHMARK_KIBANA_SECRET = 'secret/observability-team/ci/apm-benchmark-kibana'
    PNG_REPORT_FILE = 'out.png'
    TERRAFORM_VERSION = '1.1.9'
    CREATED_DATE = "${new Date().getTime()}"
    JOB_GCS_BUCKET_STASH = 'apm-ci-temp'
    JOB_GCS_CREDENTIALS = 'apm-ci-gcs-plugin'
    SLACK_CHANNEL = "#apm-server"
    TESTING_BENCHMARK_DIR = 'testing/benchmark'
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
      options {
        skipDefaultCheckout()
        retry(2)
      }
      environment {
        SSH_KEY = "./id_rsa_terraform"
        TF_VAR_private_key = "./id_rsa_terraform"
        TF_VAR_public_key = "./id_rsa_terraform.pub"
        BENCHMARK_RESULT = "benchmark-result.txt"
        TFVARS_SOURCE = "system-profiles/8GBx1zone.tfvars" // Default to use an 8gb profile
        USER = "benchci-${env.BUILD_ID}" // use build as prefix
        // cloud tags
        TF_VAR_BUILD_ID = "${env.BUILD_ID}"
        TF_VAR_ENVIRONMENT= 'ci'
        TF_VAR_BRANCH = "${env.BRANCH_NAME.toLowerCase().replaceAll('[^a-z0-9-]', '-')}"
        TF_VAR_REPO = "${REPO}"
        TF_VAR_CREATED_DATE = "${env.CREATED_DATE}"
        GOBENCH_TAGS = "branch=${BRANCH_NAME},commit=${GIT_BASE_COMMIT},pr=${CHANGE_ID},target_branch=${CHANGE_TARGET}"
      }
      steps {
        dir("${BASE_DIR}") {
          withGoEnv() {
            dir("${TESTING_BENCHMARK_DIR}") {
              withTestClusterEnv() {
                sh(label: 'Build apmbench', script: 'make apmbench $SSH_KEY terraform.tfvars')
                sh(label: 'Spin up benchmark environment', script: '$(make docker-override-committed-version) && make init apply')
                withESBenchmarkEnv() {
                  sh(label: 'Run benchmarks', script: 'make run-benchmark-autotuned index-benchmark-results')
                }
              }
            }
          }
          downloadPNGReport()
        }
      }
      post {
        always {
          dir("${BASE_DIR}/${TESTING_BENCHMARK_DIR}") {
            stashV2(name: 'benchmark_tfstate', bucket: "${JOB_GCS_BUCKET_STASH}", credentialsId: "${JOB_GCS_CREDENTIALS}")
            archiveArtifacts(artifacts: "${env.BENCHMARK_RESULT}", allowEmptyArchive: true)
          }
        }
        cleanup {
          dir("${BASE_DIR}") {
            withGoEnv() {
              dir("${TESTING_BENCHMARK_DIR}") {
                withTestClusterEnv() {
                  sh(label: 'Tear down benchmark environment', script: 'make destroy')
                }
              }
            }
          }
        }
        success {
          dir("${BASE_DIR}") {
            sendSlackReportSuccessMessage()
          }
        }
        failure {
          notifyBuildResult(slackNotify: true, slackComment: true)
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

def downloadPNGReport() {
  withSecretVault(
      secret: "${BENCHMARK_KIBANA_SECRET}",
      data: ['user': 'KIBANA_USER',
             'password': 'KIBANA_PASSWORD',
             'kibana_url': 'KIBANA_ENDPOINT']
   ) {
    sh(label: 'Run unit tests',
       script: '.ci/scripts/download-png-from-kibana.sh $KIBANA_ENDPOINT $KIBANA_USER $KIBANA_PASSWORD $PNG_REPORT_FILE')
    archiveArtifacts(allowEmptyArchive: false, artifacts: "${env.PNG_REPORT_FILE}")
  }
}

def sendSlackReportSuccessMessage() {
  withSecretVault(
      secret: "${BENCHMARK_KIBANA_SECRET}",
      data: ['kibana_dashboard_url': 'KIBANA_DASHBOARD_URL']
  ) {
    slackMessageBlocks = [
      [
        "type": "section",
        "text": [
          "type": "mrkdwn",
          "text": "Nightly benchmarks succesfully executed\n\n <${env.BUILD_URL}|Jenkins Build ${env.BUILD_DISPLAY_NAME}>"
        ]
      ],
      [
        "type": "divider"
      ],
      [
        "type": "image",
        "image_url": "${env.BUILD_URL}artifact/${env.PNG_REPORT_FILE}",
        "alt_text": "image is not downloaded"
      ],
      [
        "type": "section",
        "text": [
          "type": "mrkdwn",
          "text": "<${env.KIBANA_DASHBOARD_URL}|Kibana Dashboard>"
        ]
      ]
    ]

    slackSend(channel: "${env.SLACK_CHANNEL}", blocks: slackMessageBlocks)
  }
}
