library identifier: 'apm@master', 
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
      ansiColor('xterm')
    }
    parameters {
      string(name: 'branch_specifier', defaultValue: "", description: "the Git branch specifier to build (<branchName>, <tagName>, <commitId>, etc.)")
      string(name: 'JOB_INTEGRATION_TEST_BRANCH_SPEC', defaultValue: "refs/heads/pipeline", description: "The integrations test Git branch to use")
      string(name: 'ELASTIC_STACK_VERSION', defaultValue: "6.4", description: "Elastic Stack Git branch/tag to use")
      string(name: 'JOB_HEY_APM_TEST_BRANCH_SPEC', defaultValue: "refs/heads/master", description: "The Hey APM test Git branch/tag to use")

      booleanParam(name: 'SNAPSHOT', defaultValue: false, description: 'Build snapshot packages (defaults to true)')

      booleanParam(name: 'linux_ci', defaultValue: true, description: 'Enable Linux build')
      booleanParam(name: 'windows_cI', defaultValue: true, description: 'Enable Windows CI')
      booleanParam(name: 'intake_ci', defaultValue: true, description: 'Enable test')
      booleanParam(name: 'test_ci', defaultValue: true, description: 'Enable test')
      booleanParam(name: 'integration_test_ci', defaultValue: true, description: 'Enable run integgration test')
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
                    if(!branch_specifier){
                      echo "Checkout SCM ${GIT_BRANCH} - ${GIT_COMMIT}"
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
                    
                    /** TODO enable create tag
                    https://jenkins.io/doc/pipeline/examples/#push-git-repo
                    */
                    sh("git tag -a '${BUILD_TAG}' -m 'Jenkins TAG ${RUN_DISPLAY_URL}'")
                    sh("git push git@github.com:${ORG_NAME}/${REPO_NAME}.git --tags")
                    /*
                    withCredentials([usernamePassword(credentialsId: 'dca1b5a0-edbc-4d0e-bc0c-c38857c83a80', passwordVariable: 'GIT_PASSWORD', usernameVariable: 'GIT_USERNAME')]) {
                      sh("git tag -a '${BUILD_TAG}' -m 'Jenkins TAG ${RUN_DISPLAY_URL}'")
                      sh('git push https://${GIT_USERNAME}:${GIT_PASSWORD}@github.com/${ORG_NAME}/${REPO_NAME}.git --tags')
                    }*/
                  }
                }
                stash allowEmpty: true, name: 'source'
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
            agent { label 'linux' }
            
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
            agent { label 'linux' }
            
            when { 
              beforeAgent true
              environment name: 'linux_ci', value: 'true' 
            }
            steps {
              withEnvWrapper() {
                unstash 'source'
                dir("${BASE_DIR}"){    
                  sh """#!/bin/bash
                  ./script/jenkins/linux-build.sh
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
            agent { label 'linux' }
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
                  ./script/jenkins/linux-test.sh
                  """
                }
              }
            }
            post { 
              always { 
                /*
                publishHTML(target: [
                  allowMissing: true, 
                  keepAll: true,
                  reportDir: "${BASE_DIR}/build/coverage", 
                  reportFiles: 'full.html', 
                  reportName: 'coverage HTML v1', 
                  reportTitles: 'Coverage'])*/
                publishHTML(target: [
                    allowMissing: true, 
                    keepAll: true,
                    reportDir: "${BASE_DIR}/build", 
                    reportFiles: 'coverage-*-report.html', 
                    reportName: 'coverage HTML v2', 
                    reportTitles: 'Coverage'])
                publishCoverage(adapters: [
                  coberturaAdapter("${BASE_DIR}/build/coverage-*-report.xml")], 
                  sourceFileResolver: sourceFiles('STORE_ALL_BUILD'))
                cobertura(autoUpdateHealth: false, 
                  autoUpdateStability: false, 
                  coberturaReportFile: "${BASE_DIR}/build/coverage-*-report.xml", 
                  conditionalCoverageTargets: '70, 0, 0', 
                  failNoReports: false, 
                  failUnhealthy: false, 
                  failUnstable: false, 
                  lineCoverageTargets: '80, 0, 0', 
                  maxNumberOfBuilds: 0, 
                  methodCoverageTargets: '80, 0, 0', 
                  onlyStable: false, 
                  sourceEncoding: 'ASCII', 
                  zoomCoverageChart: false)
                archiveArtifacts(allowEmptyArchive: true, 
                  artifacts: "${BASE_DIR}/build/TEST-*.out,${BASE_DIR}/build/TEST-*.xml,${BASE_DIR}/build/junit-*.xml", 
                  onlyIfSuccessful: false)
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
            agent { label 'linux' }
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
                  copyArtifacts filter: 'bench-last.txt', fingerprintArtifacts: true, optional: true, projectName: "${JOB_NAME}", selector: lastCompleted()
                  sh """#!/bin/bash
                  go get -u golang.org/x/tools/cmd/benchcmp
                  make bench | tee bench-new.txt
                  [ -f bench-new.txt ] && cat bench-new.txt
                  [ -f bench-last.txt ] && benchcmp bench-last.txt bench-new.txt | tee bench-diff.txt 
                  [ -f bench-last.txt ] && mv bench-last.txt bench-old.txt
                  mv bench-new.txt bench-last.txt
                  """
                  archiveArtifacts allowEmptyArchive: true, artifacts: "bench-last.txt", onlyIfSuccessful: false
                  archiveArtifacts allowEmptyArchive: true, artifacts: "bench-old.txt", onlyIfSuccessful: false
                  archiveArtifacts allowEmptyArchive: true, artifacts: "bench-diff.txt", onlyIfSuccessful: false
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
        when { 
          beforeAgent true
          environment name: 'integration_test_ci', value: 'true' 
        }
        
        parallel {
          /**
           run all integration test with the commit version.
          */
          stage('Integration test') { 
            agent { label 'linux' }
            steps {
              build(
                job: 'apm-server-ci/apm-integration-testing-pipeline', 
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
            agent { label 'linux' }
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
                  withEnvBenchmarksData {
                    sh """#!/bin/bash
                    ./script/jenkins/hey-apm-test.sh
                    """
                  }
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
        agent { label 'linux' }
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
        agent { label 'linux' }
        
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
        agent { label 'linux' }
        
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
      success { 
        echo 'Success Post Actions'
        updateGithubCommitStatus(
          repoUrl: "${JOB_GIT_URL}",
          commitSha: "${JOB_GIT_COMMIT}",
          message: 'Build result SUCCESS.'
        )
      }
      aborted { 
        echo 'Aborted Post Actions'
        setGithubCommitStatus(repoUrl: "${JOB_GIT_URL}",
          commitSha: "${JOB_GIT_COMMIT}",
          message: 'Build result ABORTED.',
          state: "error")
      }
      failure { 
        echo 'Failure Post Actions'
        //step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
        setGithubCommitStatus(repoUrl: "${JOB_GIT_URL}",
          commitSha: "${JOB_GIT_COMMIT}",
          message: 'Build result FAILURE.',
          state: "failure")
      }
      unstable { 
        echo 'Unstable Post Actions'
        setGithubCommitStatus(repoUrl: "${JOB_GIT_URL}",
          commitSha: "${JOB_GIT_COMMIT}",
          message: 'Build result UNSTABLE.',
          state: "error")
      }
      always { 
        echo 'Post Actions'
        dir('cleanTags'){
          unstash 'source'
          sh("""
          git fetch --tags
          git tag -d '${BUILD_TAG}'
          git push git@github.com:${ORG_NAME}/${REPO_NAME}.git --tags
          """)
          deleteDir()
        }
      }
    }
}