#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent any
  environment {
    BASE_DIR = "src/github.com/elastic/apm-server"
    NOTIFY_TO = credentials('notify-to')
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    JOB_GCS_CREDENTIALS = 'apm-ci-gcs-plugin'
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
  }
  parameters {
    booleanParam(name: 'Run_As_Master_Branch', defaultValue: false, description: 'Allow to run any steps on a PR, some steps normally only run on master branch.')
    booleanParam(name: 'linux_ci', defaultValue: true, description: 'Enable Linux build')
    booleanParam(name: 'windows_ci', defaultValue: true, description: 'Enable Windows CI')
    booleanParam(name: 'intake_ci', defaultValue: true, description: 'Enable test')
    booleanParam(name: 'test_ci', defaultValue: true, description: 'Enable test')
    booleanParam(name: 'test_sys_env_ci', defaultValue: true, description: 'Enable system and environment test')
    booleanParam(name: 'bench_ci', defaultValue: true, description: 'Enable benchmarks')
    booleanParam(name: 'doc_ci', defaultValue: true, description: 'Enable build documentation')
    booleanParam(name: 'releaser_ci', defaultValue: true, description: 'Enable build the release packages')
  }
  stages {
    /**
     Checkout the code and stash it, to use it on other stages.
    */
    stage('Checkout') {
      agent { label 'linux && immutable' }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
      }
      options { skipDefaultCheckout() }
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}")
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
        script {
          dir("${BASE_DIR}"){
            if(env.CHANGE_TARGET){
              env.BEATS_UPDATED = sh(script: "git diff --name-only origin/${env.CHANGE_TARGET}...${env.GIT_SHA}|grep '^_beats'|wc -l",
                returnStdout: true)?.trim()
            }
          }
        }
      }
    }
    /**
    Updating generated files for Beat.
    Checks the GO environment.
    Checks the Python environment.
    Checks YAML files are generated.
    Validate that all updates were committed.
    */
    stage('Intake') {
      agent { label 'linux && immutable' }
      options { skipDefaultCheckout() }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
      }
      when {
        beforeAgent true
        expression { return params.intake_ci }
      }
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          withGoEnv(){
            sh './script/jenkins/intake.sh'
          }
        }
      }
    }
    stage('Build'){
      failFast true
      parallel {
        /**
        Build on a linux environment.
        */
        stage('linux build') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          environment {
            PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            HOME = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            expression { return params.linux_ci }
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              withGoEnv(){
                sh './script/jenkins/build.sh'
              }
            }
          }
        }
        /**
        Build on a windows environment.
        */
        stage('windows build') {
          agent { label 'windows-2019-immutable' }
          options { skipDefaultCheckout() }
          when {
            beforeAgent true
            expression { return params.windows_ci }
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              withGoEnv(){
                powershell(script: '.\\script\\jenkins\\windows-build.ps1')
              }
            }
          }
        }
      }
    }
    stage('Test') {
      failFast true
      parallel {
        /**
          Run unit tests and report junit results.
        */
        stage('Unit Test') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          environment {
            PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            HOME = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            expression { return params.test_ci }
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              withGoEnv(){
                sh './script/jenkins/unit-test.sh'
              }
            }
          }
          post {
            always {
              junit(allowEmptyResults: true,
                keepLongStdio: true,
                testResults: "${BASE_DIR}/build/junit-*.xml")
            }
          }
        }
        /**
        Runs System and Environment Tests, then generate coverage and unit test reports.
        Finally archive the results.
        */
        stage('System and Environment Tests') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          environment {
            PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            HOME = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            expression { return params.test_sys_env_ci }
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              withGoEnv(){
                sh './script/jenkins/linux-test.sh'
              }
            }
          }
          post {
            always {
              coverageReport("${BASE_DIR}/build/coverage")
              junit(allowEmptyResults: true,
                keepLongStdio: true,
                testResults: "${BASE_DIR}/build/junit-*.xml,${BASE_DIR}/build/TEST-*.xml")
              //googleStorageUpload bucket: "gs://${JOB_GCS_BUCKET}/${JOB_NAME}/${BUILD_NUMBER}", credentialsId: "${JOB_GCS_CREDENTIALS}", pathPrefix: "${BASE_DIR}", pattern: '**/build/system-tests/run/**/*', sharedPublicly: true, showInline: true
              //googleStorageUpload bucket: "gs://${JOB_GCS_BUCKET}/${JOB_NAME}/${BUILD_NUMBER}", credentialsId: "${JOB_GCS_CREDENTIALS}", pathPrefix: "${BASE_DIR}", pattern: '**/build/TEST-*.out', sharedPublicly: true, showInline: true
              tar(file: "system-tests-linux-files.tgz", archive: true, dir: "system-tests", pathPrefix: "${BASE_DIR}/build")
              tar(file: "coverage-files.tgz", archive: true, dir: "coverage", pathPrefix: "${BASE_DIR}/build")
              codecov(repo: 'apm-server', basedir: "${BASE_DIR}")
            }
          }
        }
        /**
        Run tests on a windows environment.
        Finally archive the results.
        */
        stage('windows test') {
          agent { label 'windows-2019-immutable' }
          options { skipDefaultCheckout() }
          when {
            beforeAgent true
            expression { return params.windows_ci }
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              withGoEnv(){
                powershell(script: '.\\script\\jenkins\\windows-test.ps1')
              }
            }
          }
          post {
            always {
              junit(allowEmptyResults: true,
                keepLongStdio: true,
                testResults: "${BASE_DIR}/build/junit-report.xml,${BASE_DIR}/build/TEST-*.xml")
            }
          }
        }
        /**
        Runs benchmarks on the current version and compare it with the previous ones.
        Finally archive the results.
        */
        stage('Benchmarking') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          environment {
            PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            HOME = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            allOf {
              anyOf {
                not {
                  changeRequest()
                }
                branch 'master'
                branch "\\d+\\.\\d+"
                branch "v\\d?"
                tag "v\\d+\\.\\d+\\.\\d+*"
                expression { return params.Run_As_Master_Branch }
              }
              expression { return params.bench_ci }
            }
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              withGoEnv(){
                sh './script/jenkins/bench.sh'
                sendBenchmarks(file: 'bench.out', index: "benchmark-server")
              }
            }
          }
        }
        /**
        updates beats updates the framework part and go parts of beats.
        Then build and test.
        Finally archive the results.
        */
        /*
        stage('Update Beats') {
            agent { label 'linux' }

            steps {
              ansiColor('xterm') {
                  deleteDir()
                  dir("${BASE_DIR}"){
                    unstash 'source'
                    sh """
                    #!
                    ./script/jenkins/update-beats.sh
                    """
                    archiveArtifacts allowEmptyArchive: true, artifacts: "${BASE_DIR}/build", onlyIfSuccessful: false
                  }
                }
              }
        }*/
      }
    }
    /**
    Build the documentation and archive it.
    Finally archive the results.
    */
    stage('Documentation') {
      agent { label 'linux && immutable' }
      options { skipDefaultCheckout() }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
      }
      when {
        beforeAgent true
        allOf {
          anyOf {
            branch 'master'
            expression { return params.Run_As_Master_Branch }
          }
          expression { return params.doc_ci }
        }
      }
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          withGoEnv(){
            sh """#!/bin/bash
            set -euxo pipefail
            make docs
            """
          }
        }
      }
      post{
        success {
          tar(file: "doc-files.tgz", archive: true, dir: "html_docs", pathPrefix: "${BASE_DIR}/build")
        }
      }
    }
    /**
    Checks if kibana objects are updated.
    */
    stage('Check kibana Obj. Updated') {
      agent { label 'linux && immutable' }
      options { skipDefaultCheckout() }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
      }
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          withGoEnv(){
            sh './script/jenkins/sync.sh'
          }
        }
      }
    }
    /**
      build release packages.
    */
    stage('Release') {
      agent { label 'linux && immutable' }
      options { skipDefaultCheckout() }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
      }
      when {
        beforeAgent true
        allOf {
          anyOf {
            not {
              changeRequest()
            }
            branch 'master'
            branch "\\d+\\.\\d+"
            branch "v\\d?"
            tag "v\\d+\\.\\d+\\.\\d+*"
            expression { return params.Run_As_Master_Branch }
            expression { return env.BEATS_UPDATED != "0" }
          }
          expression { return params.releaser_ci }
        }
      }
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          withGoEnv(){
            sh './script/jenkins/package.sh'
          }
        }
      }
      post {
        success {
          googleStorageUpload(bucket: "gs://${JOB_GCS_BUCKET}/snapshots",
            credentialsId: "${JOB_GCS_CREDENTIALS}",
            pathPrefix: "${BASE_DIR}/build/distributions/",
            pattern: "${BASE_DIR}/build/distributions/**/*",
            sharedPublicly: true,
            showInline: true)
        }
      }
    }
  }
  post {
    success {
      echoColor(text: '[SUCCESS]', colorfg: 'green', colorbg: 'default')
    }
    aborted {
      echoColor(text: '[ABORTED]', colorfg: 'magenta', colorbg: 'default')
    }
    failure {
      echoColor(text: '[FAILURE]', colorfg: 'red', colorbg: 'default')
      step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
    }
    unstable {
      echoColor(text: '[UNSTABLE]', colorfg: 'yellow', colorbg: 'default')
    }
  }
}
