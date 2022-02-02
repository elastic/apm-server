// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// https://apm-ci.elastic.co/job/apm-server/job/update-json-schema-mbp/

@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    HOME = "${env.WORKSPACE}"
    NOTIFY_TO = credentials('notify-to')
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    PIPELINE_LOG_LEVEL = 'INFO'
  }
  triggers {
    // Only master branch will run on a timer basis
    cron(env.BRANCH_NAME == '7.17' ? 'H H(4-5) * * 1,5' : '')
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '5', artifactNumToKeepStr: '5'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
  }
  parameters {
    booleanParam(name: 'DRY_RUN_MODE', defaultValue: false, description: 'If true, allows to execute this pipeline in dry run mode, without sending a PR.')
  }
  stages {
    stage('Checkout'){
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", repo: "git@github.com:elastic/${REPO}.git", credentialsId: "${JOB_GIT_CREDENTIALS}")
      }
    }
    stage('Update beats') {
      options { skipDefaultCheckout() }
      steps {
        dir("${BASE_DIR}"){
          withGoEnv(){
            setupAPMGitEmail(global: true)
            sh(label: 'make update-beats', script: '.ci/scripts/update-beats.sh')
          }
        }
      }
    }
    stage('Send Pull Request'){
      options { skipDefaultCheckout() }
      steps {
        dir("${BASE_DIR}"){
          createPullRequest()
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

def createPullRequest(Map args = [:]) {
  def title = '[automation] update libbeat and beats packaging'
  def message = createPRDescription()
  def labels = "automation"
  if (params.DRY_RUN_MODE) {
    log(level: 'INFO', text: "DRY-RUN: createPullRequest(labels: ${labels}, message: '${message}')")
    return
  }
  def branchName = (isPR()) ? env.CHANGE_TARGET : env.BRANCH_NAME
  if (anyChangesToBeSubmitted("${branchName}")) {
    githubCreatePullRequest(title: "${title}", labels: "${labels}", description: "${message}", base: "${branchName}")
  } else {
    log(level: 'INFO', text: "There are no changes to be submitted.")
  }
}

def anyChangesToBeSubmitted(String branch) {
  return sh(returnStatus: true, script: "git diff --quiet HEAD..${branch}") != 0
}

def createPRDescription() {
  return """### What \n Update with libbeat and beats packaging."""
}
