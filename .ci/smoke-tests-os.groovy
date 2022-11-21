#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    AWS_ACCOUNT_SECRET = 'secret/observability-team/ci/elastic-observability-aws-account-auth'
    EC_KEY_SECRET = 'secret/observability-team/ci/elastic-cloud/observability-pro'
    CREATED_DATE = "${new Date().getTime()}"
    SLACK_CHANNEL = "#apm-server"
    SMOKETEST_VERSIONS = "${params.SMOKETEST_VERSIONS}"
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
        stash(allowEmpty: true, name: 'source', useDefaultExcludes: false)
      }
    }
    stage('Smoke Tests') {
      options { skipDefaultCheckout() }
      steps {
        dir ("${BASE_DIR}") {
          echo 'TBC'
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
