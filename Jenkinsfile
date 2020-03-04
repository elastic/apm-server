#!/usr/bin/env groovy
@Library('apm@v1.0.6') _

pipeline {
  agent { label 'linux && immutable' }
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
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
        GOPATH = "${env.WORKSPACE}"
      }
      options { skipDefaultCheckout() }
      steps {
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}")
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
        script {
          dir("${BASE_DIR}"){
            env.GO_VERSION = readFile(".go-version")
            if(env.CHANGE_TARGET){
              env.BEATS_UPDATED = sh(script: "git diff --name-only origin/${env.CHANGE_TARGET}...${env.GIT_SHA}|grep '^_beats'|wc -l",
                returnStdout: true)?.trim()
            }
          }
        }
      }
    }
    stage('Build'){
      failFast true
      parallel {
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
              powershell(script: '.\\script\\jenkins\\windows-build.ps1')
            }
          }
        }
      }
    }
    stage('Test') {
      failFast true
      parallel {
        /**
        Runs System and Environment Tests, then generate coverage and unit test reports.
        Finally archive the results.
        */
        stage('System and Environment Tests') {
          options { skipDefaultCheckout() }
          environment {
            PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            HOME = "${env.WORKSPACE}"
            GOPATH = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            expression { return params.test_sys_env_ci }
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              sh './script/jenkins/linux-test.sh'
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
              powershell(script: '.\\script\\jenkins\\windows-test.ps1')
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
      }
    }
  }
}
