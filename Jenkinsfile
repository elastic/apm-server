#!/usr/bin/env groovy

pipeline {
    agent none
    environment {
        BASE_DIR="src/github.com/elastic/apm-server"
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
        string(name: 'ELASTIC_STACK_VERSION', defaultValue: "6.4", description: "Elastic Stack Git branch/tag to use")
        booleanParam(name: 'SNAPSHOT', defaultValue: false, description: 'Build snapshot packages (defaults to false)')
        booleanParam(name: 'Run_As_Master_Branch', defaultValue: false, description: 'Allow to run any steps on a PR, some steps normally only run on master branch.')
        booleanParam(name: 'linux_ci', defaultValue: true, description: 'Enable Linux build')
        booleanParam(name: 'windows_cI', defaultValue: true, description: 'Enable Windows CI')
        booleanParam(name: 'intake_ci', defaultValue: true, description: 'Enable test')
        booleanParam(name: 'test_ci', defaultValue: false, description: 'Enable test')
        booleanParam(name: 'test_sys_env_ci', defaultValue: true, description: 'Enable system and environment test')
        booleanParam(name: 'bench_ci', defaultValue: true, description: 'Enable benchmarks')
        booleanParam(name: 'doc_ci', defaultValue: true, description: 'Enable build documentation')
    }
    stages {
        /**
         Checkout the code and stash it, to use it on other stages.
         */
        stage('Checkout') {
            agent any
            environment {
                PATH = "${env.PATH}:${env.WORKSPACE}/bin"
                HOME = "${env.WORKSPACE}"
                GOPATH = "${env.WORKSPACE}"
            }
            options { skipDefaultCheckout() }
            steps {
                gitCheckout(basedir: "${BASE_DIR}")
                stash allowEmpty: true, name: 'source', useDefaultExcludes: false
                script {
                    env.GO_VERSION = readFile("${BASE_DIR}/.go-version")
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
        stage('Intake-experiment') {
            agent any
            // options { skipDefaultCheckout() }
            // environment {
            //     PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            //     HOME = "${env.WORKSPACE}"
            //     GOPATH = "${env.WORKSPACE}"
            // }
            // when {
            //     beforeAgent true
            //     environment name: 'intake_ci', value: 'true'
            // }
            steps {
                // withEnvWrapper() {
                //     unstash 'source'
                    dir("${BASE_DIR}"){
                       sh './script/jenkins/intake.sh'
                    }
                // }
            }
        }
        // stage('Build'){
        //     failFast true
        //     parallel {
        //         /**
        //          Build on a linux environment.
        //          */
        //         stage('linux build') {
        //             agent { label 'linux && immutable' }
        //             options { skipDefaultCheckout() }
        //             environment {
        //                 PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        //                 HOME = "${env.WORKSPACE}"
        //                 GOPATH = "${env.WORKSPACE}"
        //             }
        //             when {
        //                 beforeAgent true
        //                 environment name: 'linux_ci', value: 'true'
        //             }
        //             steps {
        //                 withEnvWrapper() {
        //                     unstash 'source'
        //                     dir("${BASE_DIR}"){
        //                         sh './script/jenkins/build.sh'
        //                     }
        //                 }
        //             }
        //         }
        //         /**
        //          Build on a windows environment.
        //          */
        //         stage('windows build') {
        //             agent { label 'windows' }
        //             options { skipDefaultCheckout() }
        //             when {
        //                 beforeAgent true
        //                 environment name: 'windows_ci', value: 'true'
        //             }
        //             steps {
        //                 withEnvWrapper() {
        //                     unstash 'source'
        //                     dir("${BASE_DIR}"){
        //                         powershell(script: '.\\script\\jenkins\\windows-build.ps1')
        //                     }
        //                 }
        //             }
        //         }
        //     }
        // }
        // stage('Test') {
        //     failFast true
        //     parallel {
        //         /**
        //          Run unit tests and report junit results.
        //          */
        //         stage('Unit Test') {
        //             agent { label 'linux && immutable' }
        //             options { skipDefaultCheckout() }
        //             environment {
        //                 PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        //                 HOME = "${env.WORKSPACE}"
        //                 GOPATH = "${env.WORKSPACE}"
        //             }
        //             when {
        //                 beforeAgent true
        //                 environment name: 'test_ci', value: 'true'
        //             }
        //             steps {
        //                 withEnvWrapper() {
        //                     unstash 'source'
        //                     dir("${BASE_DIR}"){
        //                         sh './script/jenkins/unit-test.sh'
        //                     }
        //                 }
        //             }
        //             post {
        //                 always {
        //                     junit(allowEmptyResults: true,
        //                             keepLongStdio: true,
        //                             testResults: "${BASE_DIR}/build/junit-*.xml")
        //                 }
        //             }
        //         }
        //         /**
        //          Runs System and Environment Tests, then generate coverage and unit test reports.
        //          Finally archive the results.
        //          */
        //         stage('System and Environment Tests') {
        //             agent { label 'linux && immutable' }
        //             options { skipDefaultCheckout() }
        //             environment {
        //                 PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        //                 HOME = "${env.WORKSPACE}"
        //                 GOPATH = "${env.WORKSPACE}"
        //             }
        //             when {
        //                 beforeAgent true
        //                 environment name: 'test_sys_env_ci', value: 'true'
        //             }
        //             steps {
        //                 withEnvWrapper() {
        //                     unstash 'source'
        //                     dir("${BASE_DIR}"){
        //                         sh './script/jenkins/linux-test.sh'
        //                     }
        //                 }
        //             }
        //             post {
        //                 always {
        //                     coverageReport("${BASE_DIR}/build/coverage")
        //                     junit(allowEmptyResults: true,
        //                             keepLongStdio: true,
        //                             testResults: "${BASE_DIR}/build/junit-*.xml,${BASE_DIR}/build/TEST-*.xml")
        //                     //googleStorageUpload bucket: "gs://${JOB_GCS_BUCKET}/${JOB_NAME}/${BUILD_NUMBER}", credentialsId: "${JOB_GCS_CREDENTIALS}", pathPrefix: "${BASE_DIR}", pattern: '**/build/system-tests/run/**/*', sharedPublicly: true, showInline: true
        //                     //googleStorageUpload bucket: "gs://${JOB_GCS_BUCKET}/${JOB_NAME}/${BUILD_NUMBER}", credentialsId: "${JOB_GCS_CREDENTIALS}", pathPrefix: "${BASE_DIR}", pattern: '**/build/TEST-*.out', sharedPublicly: true, showInline: true
        //                     tar(file: "system-tests-linux-files.tgz", archive: true, dir: "system-tests", pathPrefix: "${BASE_DIR}/build")
        //                     tar(file: "coverage-files.tgz", archive: true, dir: "coverage", pathPrefix: "${BASE_DIR}/build")
        //                     codecov(repo: 'apm-server', basedir: "${BASE_DIR}")
        //                 }
        //             }
        //         }
        //         /**
        //          Run tests on a windows environment.
        //          Finally archive the results.
        //          */
        //         stage('windows test') {
        //             agent { label 'windows' }
        //             options { skipDefaultCheckout() }
        //             when {
        //                 beforeAgent true
        //                 environment name: 'windows_ci', value: 'true'
        //             }
        //             steps {
        //                 withEnvWrapper() {
        //                     unstash 'source'
        //                     dir("${BASE_DIR}"){
        //                         powershell(script: '.\\script\\jenkins\\windows-test.ps1')
        //                     }
        //                 }
        //             }
        //             post {
        //                 always {
        //                     junit(allowEmptyResults: true,
        //                             keepLongStdio: true,
        //                             testResults: "${BASE_DIR}/build/junit-report.xml,${BASE_DIR}/build/TEST-*.xml")
        //                 }
        //             }
        //         }
        //         /**
        //          Runs benchmarks on the current version and compare it with the previous ones.
        //          Finally archive the results.
        //          */
        //         stage('Benchmarking') {
        //             agent { label 'linux && immutable' }
        //             options { skipDefaultCheckout() }
        //             environment {
        //                 PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        //                 HOME = "${env.WORKSPACE}"
        //                 GOPATH = "${env.WORKSPACE}"
        //             }
        //             when {
        //                 beforeAgent true
        //                 allOf {
        //                     anyOf {
        //                         not {
        //                             changeRequest()
        //                         }
        //                         branch 'master'
        //                         branch "\\d+\\.\\d+"
        //                         branch "v\\d?"
        //                         tag "v\\d+\\.\\d+\\.\\d+*"
        //                         environment name: 'Run_As_Master_Branch', value: 'true'
        //                     }
        //                     environment name: 'bench_ci', value: 'true'
        //                 }
        //             }
        //             steps {
        //                 withEnvWrapper() {
        //                     unstash 'source'
        //                     dir("${BASE_DIR}"){
        //                         sh './script/jenkins/bench.sh'
        //                         sendBenchmarks(file: 'bench.out', index: "benchmark-server")
        //                     }
        //                 }
        //             }
        //         }
        //         /**
        //          updates beats updates the framework part and go parts of beats.
        //          Then build and test.
        //          Finally archive the results.
        //          */
        //         /*
        //         stage('Update Beats') {
        //             agent { label 'linux' }

        //             steps {
        //               ansiColor('xterm') {
        //                   deleteDir()
        //                   dir("${BASE_DIR}"){
        //                     unstash 'source'
        //                     sh """
        //                     #!
        //                     ./script/jenkins/update-beats.sh
        //                     """
        //                     archiveArtifacts allowEmptyArchive: true, artifacts: "${BASE_DIR}/build", onlyIfSuccessful: false
        //                   }
        //                 }
        //               }
        //         }*/
        //     }
        // }
        // /**
        //  Build the documentation and archive it.
        //  Finally archive the results.
        //  */
        // stage('Documentation') {
        //     agent { label 'linux && immutable' }
        //     options { skipDefaultCheckout() }
        //     environment {
        //         PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        //         HOME = "${env.WORKSPACE}"
        //         GOPATH = "${env.WORKSPACE}"
        //     }
        //     when {
        //         beforeAgent true
        //         allOf {
        //             anyOf {
        //                 branch 'master'
        //                 environment name: 'Run_As_Master_Branch', value: 'true'
        //             }
        //             environment name: 'doc_ci', value: 'true'
        //         }
        //     }
        //     steps {
        //         withEnvWrapper() {
        //             unstash 'source'
        //             dir("${BASE_DIR}"){
        //                 sh """#!/bin/bash
        //     set -euxo pipefail
        //     make docs
        //     """
        //             }
        //         }
        //     }
        //     post{
        //         success {
        //             tar(file: "doc-files.tgz", archive: true, dir: "html_docs", pathPrefix: "${BASE_DIR}/build")
        //         }
        //     }
        // }
        // /**
        //  Checks if kibana objects are updated.
        //  */
        // stage('Check kibana Obj. Updated') {
        //     agent { label 'linux && immutable' }
        //     options { skipDefaultCheckout() }
        //     environment {
        //         PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        //         HOME = "${env.WORKSPACE}"
        //         GOPATH = "${env.WORKSPACE}"
        //     }
        //     steps {
        //         withEnvWrapper() {
        //             unstash 'source'
        //             dir("${BASE_DIR}"){
        //                 sh './script/jenkins/sync.sh'
        //             }
        //         }
        //     }
        // }
        // /**
        //  build release packages.
        //  */
        // stage('Release') {
        //     agent { label 'linux && immutable' }
        //     options { skipDefaultCheckout() }
        //     when {
        //         beforeAgent true
        //         allOf {
        //             anyOf {
        //                 not {
        //                     changeRequest()
        //                 }
        //                 branch 'master'
        //                 branch "\\d+\\.\\d+"
        //                 branch "v\\d?"
        //                 tag "v\\d+\\.\\d+\\.\\d+*"
        //                 environment name: 'Run_As_Master_Branch', value: 'true'
        //             }
        //             environment name: 'releaser_ci', value: 'true'
        //         }
        //     }
        //     steps {
        //         withEnvWrapper() {
        //             unstash 'source'
        //             dir("${BASE_DIR}"){
        //                 sh './script/jenkins/package.sh'
        //             }
        //         }
        //     }
        //     post {
        //         success {
        //             echo "Archive packages"
        //             /** TODO check if it is better storing in snapshots */
        //             //googleStorageUpload bucket: "gs://${JOB_GCS_BUCKET}/${JOB_NAME}/${BUILD_NUMBER}", credentialsId: "${JOB_GCS_CREDENTIALS}", pathPrefix: "${BASE_DIR}/build/distributions/", pattern: '${BASE_DIR}/build/distributions//**/*', sharedPublicly: true, showInline: true
        //         }
        //     }
        // }
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
            //step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
        }
        unstable {
            echoColor(text: '[UNSTABLE]', colorfg: 'yellow', colorbg: 'default')
        }
    }
}
