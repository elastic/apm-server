#!/usr/bin/env groovy
@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    REPO = 'apm-server'
    BASE_DIR = "src/github.com/elastic/${env.REPO}"
    NOTIFY_TO = credentials('notify-to')
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    JOB_GCS_CREDENTIALS = 'apm-ci-gcs-plugin'
    CODECOV_SECRET = 'secret/apm-team/ci/apm-server-codecov'
    GITHUB_CHECK_ITS_NAME = 'APM Integration Tests'
    ITS_PIPELINE = 'apm-integration-tests-selector-mbp/master'
    DIAGNOSTIC_INTERVAL = "${params.DIAGNOSTIC_INTERVAL}"
    ES_LOG_LEVEL = "${params.ES_LOG_LEVEL}"
  }
  options {
    timeout(time: 2, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '100', artifactNumToKeepStr: '30', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
    quietPeriod(10)
  }
  triggers {
    issueCommentTrigger('(?i).*(?:jenkins\\W+)?run\\W+(?:the\\W+)?tests(?:\\W+please)?.*')
  }
  parameters {
    booleanParam(name: 'Run_As_Master_Branch', defaultValue: false, description: 'Allow to run any steps on a PR, some steps normally only run on master branch.')
    booleanParam(name: 'linux_ci', defaultValue: true, description: 'Enable Linux build')
    booleanParam(name: 'windows_ci', defaultValue: true, description: 'Enable Windows CI')
    booleanParam(name: 'intake_ci', defaultValue: true, description: 'Enable test')
    booleanParam(name: 'test_ci', defaultValue: true, description: 'Enable test')
    booleanParam(name: 'test_sys_env_ci', defaultValue: true, description: 'Enable system and environment test')
    booleanParam(name: 'bench_ci', defaultValue: true, description: 'Enable benchmarks')
    booleanParam(name: 'release_ci', defaultValue: true, description: 'Enable build the release packages')
    booleanParam(name: 'kibana_update_ci', defaultValue: true, description: 'Enable build the Check kibana Obj. Updated')
    booleanParam(name: 'its_ci', defaultValue: true, description: 'Enable async ITs')
    string(name: 'DIAGNOSTIC_INTERVAL', defaultValue: "0", description: 'Elasticsearch detailed logging every X seconds')
    string(name: 'ES_LOG_LEVEL', defaultValue: "error", description: 'Elasticsearch error level')

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
        pipelineManager([ cancelPreviousRunningBuilds: [ when: 'PR' ] ])
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}", githubNotifyFirstTimeContributor: true,
                    shallow: false, reference: "/var/lib/jenkins/.git-references/${REPO}.git")
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
        script {
          dir("${BASE_DIR}"){
            env.GO_VERSION = readFile(".go-version").trim()
            def regexps =[
              "^_beats.*",
              "^apm-server.yml",
              "^apm-server.docker.yml",
              "^magefile.go",
              "^ingest.*",
              "^packaging.*",
              "^tests/packaging.*",
              "^vendor/github.com/elastic/beats.*"
            ]
            env.BEATS_UPDATED = isGitRegionMatch(patterns: regexps)

            // Skip all the stages except docs for PR's with asciidoc changes only
            env.ONLY_DOCS = isGitRegionMatch(patterns: [ '.*\\.asciidoc' ], comparator: 'regexp', shouldMatchAll: true)
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
      options { skipDefaultCheckout() }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
        GOPATH = "${env.WORKSPACE}"
      }
      when {
        beforeAgent true
        allOf {
          expression { return params.intake_ci }
          expression { return env.ONLY_DOCS == "false" }
        }
      }
      steps {
        withGithubNotify(context: 'Intake') {
          deleteDir()
          unstash 'source'
          dir("${BASE_DIR}"){
            sh(label: 'Run intake', script: './script/jenkins/intake.sh')
          }
        }
      }
    }
    stage('Build and Test'){
      failFast false
      parallel {
        /**
        Build on a linux environment.
        */
        stage('linux build') {
          options { skipDefaultCheckout() }
          when {
            beforeAgent true
            allOf {
              expression { return params.linux_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          steps {
            withGithubNotify(context: 'Build - Linux') {
              deleteDir()
              unstash 'source'
              golang(){
                dir(BASE_DIR){
                  retry(2) { // Retry in case there are any errors to avoid temporary glitches
                    sleep randomNumber(min: 5, max: 10)
                    sh(label: 'Linux build', script: './script/jenkins/build.sh')
                  }
                }
              }
            }
          }
        }
        /**
        Build and Test on a windows environment.
        */
        stage('windows build-test') {
          agent { label 'windows-2019-immutable' }
          options {
            skipDefaultCheckout()
            warnError('Windows execution failed')
          }
          when {
            beforeAgent true
            allOf {
              expression { return params.windows_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          steps {
            withGithubNotify(context: 'Build-Test - Windows') {
              deleteDir()
              unstash 'source'
              dir(BASE_DIR){
                retry(2) { // Retry in case there are any errors to avoid temporary glitches
                  sleep randomNumber(min: 5, max: 10)
                  powershell(label: 'Windows build', script: '.\\script\\jenkins\\windows-build.ps1')
                  powershell(label: 'Run Window tests', script: '.\\script\\jenkins\\windows-test.ps1')
                }
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
          Run unit tests and report junit results.
        */
        stage('Unit Test') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          environment {
            PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            HOME = "${env.WORKSPACE}"
            GOPATH = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            allOf {
              expression { return params.test_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          steps {
            withGithubNotify(context: 'Unit Tests', tab: 'tests') {
              deleteDir()
              unstash 'source'
              dir("${BASE_DIR}"){
                sh(label: 'Run Unit tests', script: './script/jenkins/unit-test.sh')
              }
            }
          }
          post {
            always {
              coverageReport("${BASE_DIR}/build/coverage")
              junit(allowEmptyResults: true,
                keepLongStdio: true,
                testResults: "${BASE_DIR}/build/junit-*.xml"
              )
              catchError(buildResult: 'SUCCESS', message: 'Failed to grab test results tar files', stageResult: 'SUCCESS') {
                tar(file: "coverage-files.tgz", archive: true, dir: "coverage", pathPrefix: "${BASE_DIR}/build")
              }
              codecov(repo: env.REPO, basedir: "${BASE_DIR}", secret: "${CODECOV_SECRET}")
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
            GOPATH = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            allOf {
              expression { return params.test_sys_env_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          steps {
            withGithubNotify(context: 'System Tests', tab: 'tests') {
              deleteDir()
              unstash 'source'
              dir("${BASE_DIR}"){
                sh(label: 'Run Linux tests', script: './script/jenkins/linux-test.sh')
              }
            }
          }
          post {
            always {
              dir("${BASE_DIR}"){
                archiveArtifacts(allowEmptyArchive: true,
                  artifacts: "docker-info/**",
                  defaultExcludes: false)
                  junit(allowEmptyResults: true,
                    keepLongStdio: true,
                    testResults: "**/build/TEST-*.xml"
                  )
              }
              catchError(buildResult: 'SUCCESS', message: 'Failed to grab test results tar files', stageResult: 'SUCCESS') {
                tar(file: "system-tests-linux-files.tgz", archive: true, dir: "system-tests", pathPrefix: "${BASE_DIR}/build")
              }
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
          when {
            beforeAgent true
            allOf {
              anyOf {
                branch 'master'
                branch pattern: '\\d+\\.\\d+', comparator: 'REGEXP'
                branch pattern: 'v\\d?', comparator: 'REGEXP'
                tag pattern: 'v\\d+\\.\\d+\\.\\d+.*', comparator: 'REGEXP'
                expression { return params.Run_As_Master_Branch }
              }
              expression { return params.bench_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          steps {
            withGithubNotify(context: 'Benchmarking') {
              deleteDir()
              unstash 'source'
              golang(){
                dir("${BASE_DIR}"){
                  sh(label: 'Run benchmarks', script: './script/jenkins/bench.sh')
                  sendBenchmarks(file: 'bench.out', index: "benchmark-server")
                }
              }
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
            GOPATH = "${env.WORKSPACE}"
          }
          when {
            beforeAgent true
            allOf {
              expression { return params.kibana_update_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          steps {
            withGithubNotify(context: 'Sync Kibana') {
              deleteDir()
              unstash 'source'
              dir("${BASE_DIR}"){
                catchError(buildResult: 'SUCCESS', message: 'Sync Kibana is not updated', stageResult: 'UNSTABLE') {
                  sh(label: 'Test Sync', script: './script/jenkins/sync.sh')
                }
              }
            }
          }
        }
      }
    }
    stage('APM Integration Tests') {
      agent none
      when {
        beforeAgent true
        allOf {
          anyOf {
            environment name: 'GIT_BUILD_CAUSE', value: 'pr'
            expression { return !params.Run_As_Master_Branch }
          }
          expression { return params.its_ci }
          expression { return env.ONLY_DOCS == "false" }
        }
      }
      steps {
        log(level: 'INFO', text: "Launching Async ${env.GITHUB_CHECK_ITS_NAME}")
        build(job: env.ITS_PIPELINE, propagate: false, wait: false,
              parameters: [string(name: 'AGENT_INTEGRATION_TEST', value: 'All'),
                           string(name: 'BUILD_OPTS', value: "--apm-server-build https://github.com/elastic/${env.REPO}@${env.GIT_BASE_COMMIT}"),
                           string(name: 'GITHUB_CHECK_NAME', value: env.GITHUB_CHECK_ITS_NAME),
                           string(name: 'GITHUB_CHECK_REPO', value: env.REPO),
                           string(name: 'GITHUB_CHECK_SHA1', value: env.GIT_BASE_COMMIT)])
        githubNotify(context: "${env.GITHUB_CHECK_ITS_NAME}", description: "${env.GITHUB_CHECK_ITS_NAME} ...", status: 'PENDING', targetUrl: "${env.JENKINS_URL}search/?q=${env.ITS_PIPELINE.replaceAll('/','+')}")
      }
    }
    /**
      build release packages.
    */
    stage('Release') {
      options { skipDefaultCheckout() }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
        GOPATH = "${env.WORKSPACE}"
        SNAPSHOT="true"
      }
      when {
        beforeAgent true
        allOf {
          anyOf {
            branch 'master'
            branch pattern: '\\d+\\.\\d+', comparator: 'REGEXP'
            branch pattern: 'v\\d?', comparator: 'REGEXP'
            tag pattern: 'v\\d+\\.\\d+\\.\\d+.*', comparator: 'REGEXP'
            expression { return params.Run_As_Master_Branch }
            expression { return env.BEATS_UPDATED != "false" }
          }
          expression { return params.release_ci }
          expression { return env.ONLY_DOCS == "false" }
        }
      }
      steps {
        withGithubNotify(context: 'Release') {
          deleteDir()
          unstash 'source'
          /**
            The package build needs mage and docker
          */
          golang(){
            dir("${BASE_DIR}"){
              sh(label: 'Build packages', script: './script/jenkins/package.sh')
              sh(label: 'Test packages install', script: './script/jenkins/test-install-packages.sh')
            }
          }
        }
      }
      post {
        success {
          echo "Archive packages"
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
    cleanup {
      notifyBuildResult()
    }
  }
}

def golang(Closure body){
  def golangDocker
  retry(3) { // Retry in case there are any errors when building the docker images (to avoid temporary glitches)
    sleep randomNumber(min: 2, max: 5)
    golangDocker = docker.build('golang-mage', "--build-arg GO_VERSION=${GO_VERSION} ${BASE_DIR}/.ci/docker/golang-mage")
  }
  withEnv(["HOME=${WORKSPACE}", "GOPATH=${WORKSPACE}", "SHELL=/bin/bash"]) {
     golangDocker.inside('-v /usr/bin/docker:/usr/bin/docker -v /var/run/docker.sock:/var/run/docker.sock'){
       body()
     }
   }
}
