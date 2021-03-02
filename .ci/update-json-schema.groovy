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
    cron('H H(4-5) * * 1,5')
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
    booleanParam(name: 'FORCE_SEND_PR', defaultValue: false, description: 'If true, will force sending a PR, although it could be affected by the value off the DRY_RUN parameter: if the latter is true, a message will be printed in the console.')
    choice(name: 'APM_AGENTS', choices: [
                                       'All',
                                       '.NET',
                                       'Go',
                                       'Java',
                                       'Node.js',
                                       'PHP',
                                       'Python',
                                       'Ruby',
                                       'RUM'], description: 'Name of the APM Agent you want to update its specs.')
  }
  stages {
    stage('Checkout'){
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}",
          repo: "git@github.com:elastic/${REPO}.git",
          credentialsId: "${JOB_GIT_CREDENTIALS}"
        )
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
      }
    }
    // This stage will populate the environment, and will only be executed under any of the
    // following conditions:
    // 1. we run the pipeline NOT in DRY_RUN_MODE, because we want to send real PRs
    // 2. we run the pipeline forcing sending real PRs, because we want so
    // Because the rest of the following stages will need these variables to check for changes,
    // skipping this stage would not take effect in them, as they are covered by the 
    // FORCE_SEND_PR check.
    stage('Check for schema changes'){
      when {
        beforeAgent true
        anyOf {
          expression { return env.DRY_RUN_MODE == "false" }
          expression { return params.FORCE_SEND_PR }
        }
      }
      environment {
        // GIT_PREVIOUS_SUCCESSFUL_COMMIT might point to a local merge commit instead a commit in the
        // origin, then let's use the target branch for PRs and the GIT_PREVIOUS_SUCCESSFUL_COMMIT for
        // branches.
        COMMIT_FROM = """${isPR() ? "origin/${env.CHANGE_TARGET}" : "${env.GIT_PREVIOUS_SUCCESSFUL_COMMIT}"}"""
      }
      steps {
        deleteDir()
        unstash 'source'
        script {
          dir("${BASE_DIR}"){
            regexps = [ "^docs/spec/v2/.*" ]
            env.SPECS_UPDATED = isGitRegionMatch(
              from: "${env.COMMIT_FROM}",
              patterns: regexps)
            env.PR_DESCRIPTION = createPRDescription(env.COMMIT_FROM)
          }
        }
      }
    }
    stage('Send Pull Request for JSON specs'){
      options {
        warnError('Pull Requests to APM agents failed')
      }
      when {
        beforeAgent true
        anyOf {
          expression { return env.SPECS_UPDATED == "true" }
          expression { return params.FORCE_SEND_PR }
        }
      }
      environment {
        // agentMapping is defined in the shared library as a map
        APM_AGENTS = "${params?.APM_AGENTS}"
        SELECTED_AGENT = agentMapping.id(env.APM_AGENTS)
      }
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          generateSteps()
        }
      }
    }
  }
  post {
    cleanup {
      // PR comments should only be created for the main pipeline.
      notifyBuildResult(prComment: false)
    }
  }
}

def generateSteps() {
  def agents = readYaml(file: '.ci/.jenkins-schema.yml')
  def parallelTasks = [:]
  agents['agents'].each { agent ->
    if (agent.SPEC_FILEPATH?.trim()) {
      if (env.SELECTED_AGENT == 'all' || "apm-agent-${env.SELECTED_AGENT}" == agent.REPO) {
        parallelTasks["${agent.REPO}"] = generateStepForAgent(repo: "${agent.REPO}", filePath: "${agent.SPEC_FILEPATH}")
      }
    }
  }
  parallel(parallelTasks)
}

def generateStepForAgent(Map args = [:]){
  def repo = args.containsKey('repo') ? args.get('repo') : error('generateStepForAgent: repo argument is required')
  def filePath = args.containsKey('filePath') ? args.get('filePath') : error('generateStepForAgent: filePath argument is required')
  return {
    node('linux && immutable') {
      catchError(buildResult: 'SUCCESS', stageResult: 'UNSTABLE') {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          setupAPMGitEmail(global: true)
          sh script: """.ci/scripts/prepare-spec-changes.sh "${repo}" "${filePath}" """, label: "Prepare changes for ${repo}"
          dir(".ci/${repo}") {
            if (params.DRY_RUN_MODE || isPR()) {
              echo "DRY-RUN: ${repo} with description: '${env.PR_DESCRIPTION}'"
            } else {
              githubCreatePullRequest(title: "synchronize schema spec", labels: 'automation', description: "${env.PR_DESCRIPTION}")
            }
          }
        }
      }
    }
  }
}

def createPRDescription(commit) {
  def message = """
  ### What
  APM agent json schema automatic sync

  ### Why
  """
  if (params.FORCE_SEND_PR) {
    message += "*Manually forced with the CI automation job.*"
  }
  if (env?.SPECS_UPDATED?.equals('true')){
    def gitLog = sh(script: """
      git log --pretty=format:'* https://github.com/${env.ORG_NAME}/${env.REPO_NAME}/commit/%h %s' \
          ${commit}...HEAD \
          --follow -- spec \
      | sed 's/#\\([0-9]\\+\\)/https:\\/\\/github.com\\/${env.ORG_NAME}\\/${env.REPO_NAME}\\/pull\\/\\1/g' || true""", returnStdout: true)
    message += "*Changeset*\n${gitLog}"
  }
  return message
}
