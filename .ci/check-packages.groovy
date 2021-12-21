#!/usr/bin/env groovy

@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    BASE_DIR = 'src'
    PIPELINE_LOG_LEVEL = 'INFO'
    HOME = "${WORKSPACE}"
    // This limits ourselves to just the APM tests
    ANSIBLE_EXTRA_FLAGS = "--tags apm-server"
    LANG = "C.UTF-8"
    LC_ALL = "C.UTF-8"
    PYTHONUTF8 = "1"
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
    upstream("apm-server/apm-server-mbp/${env.JOB_BASE_NAME}")
  }
  parameters {
    string(name: 'APM_URL_BASE', defaultValue: 'https://storage.googleapis.com/apm-ci-artifacts/jobs/snapshots', description: 'The location where the APM packages should be downloaded from')
    string(name: 'VERSION', defaultValue: '8.0.0-SNAPSHOT', description: 'The package version to test (modify the job configuration to add a new version)')
  }
  stages {
    stage('Checkout') {
      options { skipDefaultCheckout() }
      steps {
        pipelineManager([ cancelPreviousRunningBuilds: [ when: 'PR' ] ])
        deleteDir()
        script {
          if(isUpstreamTrigger()) {
            try {
              log(level: 'INFO', text: "Started by upstream pipeline. Read 'beats-tester.properties'.")
              copyArtifacts(filter: 'beats-tester.properties',
                            flatten: true,
                            projectName: "apm-server/apm-server-mbp/${env.JOB_BASE_NAME}",
                            selector: upstream(fallbackToLastSuccessful: true))
              def props = readProperties(file: 'beats-tester.properties')
              setEnvVar('APM_URL_BASE', props.get('APM_URL_BASE', ''))
              setEnvVar('VERSION', props.get('VERSION', '8.0.0-SNAPSHOT'))
            } catch(err) {
              log(level: 'WARN', text: "copyArtifacts failed. Fallback to the head of the branch as used to be.")
              setEnvVar('APM_URL_BASE', params.get('APM_URL_BASE', 'https://storage.googleapis.com/apm-ci-artifacts/jobs/snapshots'))
              setEnvVar('VERSION', params.get('VERSION', '8.0.0-SNAPSHOT'))
            }
          } else {
            log(level: 'INFO', text: "No started by upstream pipeline. Fallback to the head of the branch as used to be.")
            setEnvVar('APM_URL_BASE', params.get('APM_URL_BASE'))
            setEnvVar('VERSION', params.get('VERSION'))
          }
        }
        gitCheckout(basedir: "${BASE_DIR}", repo: 'git@github.com:elastic/beats-tester.git', branch: 'main', credentialsId: 'f6c7695a-671e-4f4f-a331-acdce44ff9ba')
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
      }
    }
    stage('Test Hosts'){
      matrix {
        // TODO: when the infra is ready with the 'nested-virtualization' then we can use that label
        // agent { label 'nested-virtualization' }
        agent { label 'metal' }
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
              deleteDir()
              unstash 'source'
              dir("${BASE_DIR}"){
                withGoEnv(os: 'linux'){
                  sh(label: 'make batch',
                    script: """#!/bin/bash
                      echo "apm_url_base: ${APM_URL_BASE}" > run-settings-jenkins.yml
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
                  withGoEnv(os: 'linux'){
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
      notifyBuildResult(prComment: false)
    }
  }
}
