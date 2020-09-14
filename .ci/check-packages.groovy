#!/usr/bin/env groovy

@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    BASE_DIR = 'src'
    PIPELINE_LOG_LEVEL = 'INFO'
    URL_BASE = "${params.URL_BASE}"
    VERSION = "${params.VERSION}"
    HOME = "${WORKSPACE}"
    // This limits ourselves to just the APM tests
    ANSIBLE_EXTRA_FLAGS = "--tags apm-server"
    // The build parameters
    BEATS_URL_BASE = 'https://storage.googleapis.com/beats-ci-artifacts/snapshots'
    APM_URL_BASE = 'https://storage.googleapis.com/apm-ci-artifacts/jobs/snapshots'
    // BRANCH_NAME = 'master'
  }
  options {
    timeout(time: 4, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    quietPeriod(10)
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
  }
  triggers {
    cron '@weekly'
  }
  stages {
    stage('Checkout') {
      options { skipDefaultCheckout() }
      steps {
        pipelineManager([ cancelPreviousRunningBuilds: [ when: 'PR' ] ])
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", repo: 'git@github.com:elastic/beats-tester.git', branch: 'master', credentialsId: 'f6c7695a-671e-4f4f-a331-acdce44ff9ba')
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
      }
    }
    stage('Test Hosts'){
      matrix {
        // TODO: when the infra is ready with the 'nested-virtualization' then we can use that label
        // agent { label 'nested-virtualization' }
        agent { label 'darwin' }
        axes {
          axis {
            name 'GROUPS'
            // TODO: when the infra is ready with the 'nested-virtualization' then we can split in stages
            // values 'centos', 'debian', 'sles', 'windows'
            values 'centos debian sles windows'
          }
        }
        stages {
          stage('Test'){
            options { skipDefaultCheckout() }
            steps {
              // See https://stackoverflow.com/questions/59269208/errorrootcode-for-hash-md5-was-not-found-when-using-any-hg-mercurial-command
              sh(label: "Switching OpenSSL versions to fix Py2", script: "brew switch openssl 1.0.2s")
              deleteDir()
              unstash 'source'
              dir("${BASE_DIR}"){
                withGoEnv(){
                  sh(label: 'make batch',
                    script: """#!/bin/bash
                      echo "beats_url_base: ${BEATS_URL_BASE}" > run-settings-jenkins.yml
                      echo "apm_url_base: ${APM_URL_BASE}" >> run-settings-jenkins.yml
                      echo "version: ${VERSION}" >> run-settings-jenkins.yml
                      RUN_SETTINGS=jenkins make batch""")
                }
              }
            }
            post {
              always {
                dir("${BASE_DIR}"){
                  junit(allowEmptyResults: true, keepLongStdio: true, testResults: "logs/*.xml")
                  archiveArtifacts(allowEmptyArchive: true, artifacts: 'logs/**')
                  withGoEnv(){
                    sh(label: 'make clean', script: 'make clean')
                  }
                }
              }
              cleanup {
                deleteDir()
              }
            }
          }
        }
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: true)
    }
  }
}
