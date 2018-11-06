#!/usr/bin/env groovy

library identifier: 'apm@master',
changelog: false,
retriever: modernSCM(
  [$class: 'GitSCMSource', 
  credentialsId: 'f6c7695a-671e-4f4f-a331-acdce44ff9ba', 
  remote: 'git@github.com:elastic/apm-pipeline-library.git'])

pipeline {
  agent any
  environment {
    HOME = "${env.HUDSON_HOME}"
    BASE_DIR="src/github.com/elastic/apm-server"
    JOB_GIT_INTEGRATION_URL="git@github.com:elastic/apm-integration-testing.git"
    INTEGRATION_TEST_BASE_DIR = "src/github.com/elastic/apm-integration-testing"
    HEY_APM_TEST_BASE_DIR = "src/github.com/elastic/hey-apm"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
  }
  triggers {
    cron('0 0 * * 1-5')
    githubPush()
  }
  options {
    timeout(time: 1, unit: 'HOURS') 
    buildDiscarder(logRotator(numToKeepStr: '3', artifactNumToKeepStr: '2', daysToKeepStr: '30'))
    timestamps()
    preserveStashes()
    //ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
  }
  parameters {
    string(name: 'branch_specifier', defaultValue: "", description: "the Git branch specifier to build (<branchName>, <tagName>, <commitId>, etc.)")
    string(name: 'GO_VERSION', defaultValue: "1.10.3", description: "Go version to use.")
    string(name: 'JOB_INTEGRATION_TEST_BRANCH_SPEC', defaultValue: "refs/heads/pipeline", description: "The integrations test Git branch to use")
    string(name: 'ELASTIC_STACK_VERSION', defaultValue: "6.4", description: "Elastic Stack Git branch/tag to use")
    string(name: 'JOB_HEY_APM_TEST_BRANCH_SPEC', defaultValue: "refs/heads/master", description: "The Hey APM test Git branch/tag to use")

    booleanParam(name: 'SNAPSHOT', defaultValue: false, description: 'Build snapshot packages (defaults to true)')

    booleanParam(name: 'linux_ci', defaultValue: true, description: 'Enable Linux build')
    booleanParam(name: 'windows_cI', defaultValue: true, description: 'Enable Windows CI')
    booleanParam(name: 'intake_ci', defaultValue: true, description: 'Enable test')
    booleanParam(name: 'test_ci', defaultValue: true, description: 'Enable test')
    booleanParam(name: 'integration_test_ci', defaultValue: false, description: 'Enable run integration test')
    booleanParam(name: 'hey_apm_ci', defaultValue: false, description: 'Enable run integration test')
    booleanParam(name: 'bench_ci', defaultValue: true, description: 'Enable benchmarks')
    booleanParam(name: 'doc_ci', defaultValue: true, description: 'Enable build documentation')
  }
  
  stages {
    
    /**
     Checkout the code and stash it, to use it on other stages.
    */
    stage('Checkout') {
      agent { label 'master || linux' }
      environment {
        PATH = "${env.PATH}:${env.HUDSON_HOME}/go/bin/:${env.WORKSPACE}/bin"
        GOPATH = "${env.WORKSPACE}"
      }
      
      steps {
          withEnvWrapper() {
              dir("${BASE_DIR}"){
                script{
                  if(!env?.branch_specifier){
                    echo "Checkout SCM"
                    checkout scm
                  } else {
                    echo "Checkout ${branch_specifier}"
                    checkout([$class: 'GitSCM', branches: [[name: "${branch_specifier}"]], 
                      doGenerateSubmoduleConfigurations: false, 
                      extensions: [], 
                      submoduleCfg: [], 
                      userRemoteConfigs: [[credentialsId: "${JOB_GIT_CREDENTIALS}", 
                      url: "${GIT_URL}"]]])
                  }
                  env.JOB_GIT_COMMIT = getGitCommitSha()
                  env.JOB_GIT_URL = "${GIT_URL}"
                  
                  github_enterprise_constructor()
                  
                  on_change{
                    echo "build cause a change (commit or PR)"
                  }
                  
                  on_commit {
                    echo "build cause a commit"
                  }
                  
                  on_merge {
                    echo "build cause a merge"
                  }
                  
                  on_pull_request {
                    echo "build cause PR"
                  }
                }
              }
              stash allowEmpty: true, name: 'source', useDefaultExcludes: false
          }
      }
    }
    
    stage('Parallel Builds'){
      failFast true
      parallel {
        
        /**
        Updating generated files for Beat.
        Checks the GO environment.
        Checks the Python environment.
        Checks YAML files are generated. 
        Validate that all updates were committed.
        */
        stage('Intake') { 
          agent { label 'linux && immutable' }
          
          when { 
            beforeAgent true
            allOf { 
              branch 'master';
              environment name: 'intake_ci', value: 'true' 
            }
          }
          steps {
            withEnvWrapper() {
              unstash 'source'
              dir("${BASE_DIR}"){
                sh """#!/bin/bash
                ./script/jenkins/intake.sh
                """
              }
            }
          }
        }
        
        /**
        Build on a linux environment.
        */
        stage('linux build') { 
          agent { label 'linux && immutable' }
          
          when { 
            beforeAgent true
            environment name: 'linux_ci', value: 'true' 
          }
          steps {
            withEnvWrapper() {
              unstash 'source'
              dir("${BASE_DIR}"){    
                sh """#!/bin/bash
                ./script/jenkins/build.sh
                """
              }
            }
          }
        }
        
        /**
        Build a windows environment.
        */
        stage('windows build') { 
          agent { label 'windows' }
          
          when { 
            beforeAgent true
            environment name: 'windows_ci', value: 'true' 
          }
          steps {
            withEnvWrapper() {
              unstash 'source'
              dir("${BASE_DIR}"){  
                /*
                powershell '''java -jar "C:\\Program Files\\infra\\bin\\runbld" `
                  --program powershell.exe `
                  --args "-NonInteractive -ExecutionPolicy ByPass -File" `
                  ".\\script\\jenkins\\windows-build.ps1"'''
                  */
                  bat 'dir "C:\\Program Files\\java'
                  powershell '".\\script\\jenkins\\windows-build.ps1"'
              }
            }
          }
        }
      } 
    }
      
    stage('Parallel Tests') {
      failFast true
      parallel {
        
        /**
        Runs unit test, then generate coverage and unit test reports.
        Finally archive the results.
        */
        stage('Linux test') { 
          agent { label 'linux && immutable' }
          environment {
            PATH = "${env.PATH}:${env.HUDSON_HOME}/go/bin/:${env.WORKSPACE}/bin"
            GOPATH = "${env.WORKSPACE}"
          }
          
          when { 
            beforeAgent true
            environment name: 'test_ci', value: 'true' 
          }
          steps {
            withEnvWrapper() {
              unstash 'source'
              dir("${BASE_DIR}"){
                sh """#!/bin/bash
                ./script/jenkins/test.sh
                """
                codecov('apm-server')
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
            }
          }
        }
        
        /**
        Build and run tests on a windows environment.
        Finally archive the results.
        */
        stage('windows test') { 
          agent { label 'windows' }
          
          when { 
            beforeAgent true
            environment name: 'windows_ci', value: 'true' 
          }
          steps {
            withEnvWrapper() {
              unstash 'source'
              dir("${BASE_DIR}"){  
                /*
                powershell '''java -jar "C:\\Program Files\\infra\\bin\\runbld" `
                  --program powershell.exe `
                  --args "-NonInteractive -ExecutionPolicy ByPass -File" `
                  ".\\script\\jenkins\\windows-test.ps1"'''
                  */
                powershell '".\\script\\jenkins\\windows-test.ps1"'
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
          environment {
            PATH = "${env.PATH}:${env.HUDSON_HOME}/go/bin/:${env.WORKSPACE}/bin"
            GOPATH = "${env.WORKSPACE}"
          }
          
          when { 
            beforeAgent true
            environment name: 'bench_ci', value: 'true' 
          }
          steps {
            withEnvWrapper() {
              unstash 'source'
              dir("${BASE_DIR}"){  
                sh """#!/bin/bash
                ./script/jenkins/bench.sh
                """
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

    stage('Integration Tests') {
      failFast true
      parallel {
        /**
         run all integration test with the commit version.
        */
        stage('Integration test') { 
          agent { label 'linux && immutable' }
          when { 
            beforeAgent true
            environment name: 'integration_test_ci', value: 'true' 
          }
          steps {
            build(
              job: 'apm-server-ci/apm-integration-test-pipeline', 
              parameters: [
                string(name: 'JOB_INTEGRATION_TEST_BRANCH_SPEC', value: "${JOB_INTEGRATION_TEST_BRANCH_SPEC}"), 
                string(name: 'ELASTIC_STACK_VERSION', value: "${ELASTIC_STACK_VERSION}"), 
                string(name: 'APM_SERVER_BRANCH', value: "${BUILD_TAG}"),
                string(name: 'BUILD_DESCRIPTION', value: "${BUILD_TAG}-INTEST"),
                booleanParam(name: "go_Test", value: true),
                booleanParam(name: "java_Test", value: true),
                booleanParam(name: "ruby_Test", value: true),
                booleanParam(name: "python_Test", value: true),
                booleanParam(name: "nodejs_Test", value: true),
                booleanParam(name: "kibana_Test", value: true),
                booleanParam(name: "server_Test", value: true)],
              wait: true,
              propagate: true)
          }
        }

        /**
          Unit tests and apm-server stress testing.
        */
        stage('Hey APM test') { 
          agent { label 'linux && immutable' }
          when { 
            beforeAgent true
            environment name: 'hey_apm_ci', value: 'true' 
          }
          steps {
            withEnvWrapper() {
              unstash 'source'
              dir("${HEY_APM_TEST_BASE_DIR}"){
                checkout([$class: 'GitSCM', branches: [[name: "${JOB_HEY_APM_TEST_BRANCH_SPEC}"]], 
                  doGenerateSubmoduleConfigurations: false, 
                  extensions: [], 
                  submoduleCfg: [], 
                  userRemoteConfigs: [[credentialsId: "${JOB_GIT_CREDENTIALS}", 
                  url: "https://github.com/elastic/hey-apm.git"]]])
              }
              dir("${BASE_DIR}"){
                sh """#!/bin/bash
                ./script/jenkins/hey-apm-test.sh
                """
              }
            }  
          }
        }
      }
    }
    
    /**
    Build the documentation and archive it.
    Finally archive the results.
    */
    stage('Documentation') { 
      agent { label 'linux && immutable' }
      environment {
        PATH = "${env.PATH}:${env.HUDSON_HOME}/go/bin/:${env.WORKSPACE}/bin"
        GOPATH = "${env.WORKSPACE}"
      }
      
      when { 
        beforeAgent true
        environment name: 'doc_ci', value: 'true' 
      }
      steps {
        withEnvWrapper() {
          unstash 'source'
          dir("${BASE_DIR}"){  
            sh """#!/bin/bash
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
    
    stage('Release') { 
      agent { label 'linux && immutable' }
      
      when { 
        beforeAgent true
        environment name: 'releaser_ci', value: 'true' 
      }
      steps {
        withEnvWrapper() {
          unstash 'source'
          dir("${BASE_DIR}"){
            sh """#!/bin/bash
            ./script/jenkins/package.sh
            """
          }
        }  
      }
      post {
        success {
          echo "Archive packages"
          /** TODO check if it is better storing in snapshots */
          //googleStorageUpload bucket: "gs://${JOB_GCS_BUCKET}/${JOB_NAME}/${BUILD_NUMBER}", credentialsId: "${JOB_GCS_CREDENTIALS}", pathPrefix: "${BASE_DIR}/build/distributions/", pattern: '${BASE_DIR}/build/distributions//**/*', sharedPublicly: true, showInline: true
        }
      }
    }

    /**
    Checks if kibana objects are updated.
    */
    stage('Check kibana Obj. Updated') { 
      agent { label 'linux && immutable' }
      
      when { 
        beforeAgent true
        branch 'master' 
      }
      steps {
      withEnvWrapper() {
          unstash 'source'
          dir("${BASE_DIR}"){  
            sh """#!/bin/bash 
            ./script/jenkins/sync.sh
            """
          }
        }
      }
    }
  }
  post { 
    always { 
      echo 'Post Actions'
    }
    success { 
      echo 'Success Post Actions'
    }
    aborted { 
      echo 'Aborted Post Actions'
    }
    failure { 
      echo 'Failure Post Actions'
      //step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
    }
    unstable { 
      echo 'Unstable Post Actions'
    }
  }
}