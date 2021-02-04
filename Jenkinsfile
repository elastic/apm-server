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
    ITS_PIPELINE = 'apm-integration-tests-selector-mbp/7.x'
    DIAGNOSTIC_INTERVAL = "${params.DIAGNOSTIC_INTERVAL}"
    ES_LOG_LEVEL = "${params.ES_LOG_LEVEL}"
    DOCKER_SECRET = 'secret/apm-team/ci/docker-registry/prod'
    DOCKER_REGISTRY = 'docker.elastic.co'
    DOCKER_IMAGE = "${env.DOCKER_REGISTRY}/observability-ci/apm-server"
    ONLY_DOCS = "false"
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
    issueCommentTrigger('(?i)(.*(?:jenkins\\W+)?run\\W+(?:the\\W+)?(?:(hey-apm|package)\\W+)?tests(?:\\W+please)?.*|^\\/test|^\\/hey-apm|^\\/package)')
  }
  parameters {
    booleanParam(name: 'Run_As_Master_Branch', defaultValue: false, description: 'Allow to run any steps on a PR, some steps normally only run on master branch.')
    booleanParam(name: 'linux_ci', defaultValue: true, description: 'Enable Linux build')
    booleanParam(name: 'osx_ci', defaultValue: true, description: 'Enable OSX CI')
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
            setEnvVar('APM_SERVER_VERSION', sh(label: 'Get beat version', script: 'make get-version', returnStdout: true)?.trim())
            env.BEATS_UPDATED = isGitRegionMatch(patterns: regexps)
            // Skip all the stages except docs for PR's with asciidoc changes only
            whenTrue(isPR()) {
              setEnvVar('ONLY_DOCS', isGitRegionMatch(patterns: [ '.*\\.asciidoc' ], comparator: 'regexp', shouldMatchAll: true))
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
            sh(label: 'Run intake', script: './.ci/scripts/intake.sh')
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
                    sh(label: 'Linux build', script: './.ci/scripts/build.sh')
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
                  powershell(label: 'Windows build', script: '.\\.ci\\scripts\\windows-build.ps1')
                  powershell(label: 'Run Window tests', script: '.\\.ci\\scripts\\windows-test.ps1')
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
        Build on a mac environment.
        */
        stage('OSX build-test') {
          agent { label 'macosx' }
          options {
            skipDefaultCheckout()
            warnError('OSX execution failed')
          }
          when {
            beforeAgent true
            allOf {
              expression { return params.osx_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          environment {
            HOME = "${env.WORKSPACE}"
          }
          steps {
            withGithubNotify(context: 'Build-Test - OSX') {
              deleteDir()
              unstash 'source'
              dir(BASE_DIR){
                retry(2) { // Retry in case there are any errors to avoid temporary glitches
                  sleep randomNumber(min: 5, max: 10)
                  sh(label: 'OSX build', script: '.ci/scripts/build-darwin.sh')
                  sh(label: 'Run Unit tests', script: '.ci/scripts/test-darwin.sh')
                }
              }
            }
          }
          post {
            always {
              junit(allowEmptyResults: true, keepLongStdio: true, testResults: "${BASE_DIR}/build/junit-*.xml")
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
                sh(label: 'Run Unit tests', script: './.ci/scripts/unit-test.sh')
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
                sh(label: 'Run Linux tests', script: './.ci/scripts/linux-test.sh')
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
                  sh(label: 'Run benchmarks', script: './.ci/scripts/bench.sh')
                }
              }
              sendBenchmarks(file: "${BASE_DIR}/bench.out", index: "benchmark-server")
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
                  sh(label: 'Test Sync', script: './.ci/scripts/sync.sh')
                }
              }
            }
          }
        }
        stage('Hey-Apm') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          when {
            beforeAgent true
            expression { return env.GITHUB_COMMENT?.contains('hey-apm tests') || env.GITHUB_COMMENT?.contains('/hey-apm')}
          }
          steps {
            withGithubNotify(context: 'Hey-Apm') {
              deleteDir()
              unstash 'source'
              golang(){
                dockerLogin(secret: env.DOCKER_SECRET, registry: env.DOCKER_REGISTRY)
                dir("${BASE_DIR}"){
                  sh(label: 'Package & Push', script: "./.ci/scripts/package-docker-snapshot.sh ${env.GIT_BASE_COMMIT} ${env.DOCKER_IMAGE}")
                }
              }
              build(job: 'apm-server/apm-hey-test-benchmark', propagate: true, wait: true,
                    parameters: [string(name: 'GO_VERSION', value: '1.12.1'),
                                 string(name: 'STACK_VERSION', value: "${env.GIT_BASE_COMMIT}"),
                                 string(name: 'APM_DOCKER_IMAGE', value: "${env.DOCKER_IMAGE}")])
            }
          }
        }
        stage('Package') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          environment {
            PATH = "${env.PATH}:${env.WORKSPACE}/bin"
            HOME = "${env.WORKSPACE}"
            GOPATH = "${env.WORKSPACE}"
            SNAPSHOT = "true"
          }
          when {
            beforeAgent true
            allOf {
              expression { return params.release_ci }
              expression { return env.ONLY_DOCS == "false" }
              anyOf {
                branch 'master'
                branch pattern: '\\d+\\.\\d+', comparator: 'REGEXP'
                tag pattern: 'v\\d+\\.\\d+\\.\\d+.*', comparator: 'REGEXP'
                expression { return isPR() && env.BEATS_UPDATED != "false" }
                expression { return env.GITHUB_COMMENT?.contains('package tests') || env.GITHUB_COMMENT?.contains('/package')}
                expression { return params.Run_As_Master_Branch }
              }
            }
          }
          stages {
            stage('Package') {
              steps {
                withGithubNotify(context: 'Package') {
                  deleteDir()
                  unstash 'source'
                  golang(){
                    dir("${BASE_DIR}"){
                      sh(label: 'Build packages', script: './.ci/scripts/package.sh')
                      sh(label: 'Test packages install', script: './.ci/scripts/test-install-packages.sh')
                      dockerLogin(secret: env.DOCKER_SECRET, registry: env.DOCKER_REGISTRY)
                      sh(label: 'Package & Push', script: "./.ci/scripts/package-docker-snapshot.sh ${env.GIT_BASE_COMMIT} ${env.DOCKER_IMAGE}")
                    }
                  }
                }
              }
            }
            stage('Publish') {
              environment {
                BUCKET_URI = """${isPR() ? "gs://${JOB_GCS_BUCKET}/pull-requests/pr-${env.CHANGE_ID}" : "gs://${JOB_GCS_BUCKET}/snapshots"}"""
              }
              steps {
                // Upload files to the default location
                googleStorageUpload(bucket: "${BUCKET_URI}",
                  credentialsId: "${JOB_GCS_CREDENTIALS}",
                  pathPrefix: "${BASE_DIR}/build/distributions/",
                  pattern: "${BASE_DIR}/build/distributions/**/*",
                  sharedPublicly: true,
                  showInline: true)

                // Copy those files to another location with the sha commit to test them afterward.
                googleStorageUpload(bucket: "gs://${JOB_GCS_BUCKET}/commits/${env.GIT_BASE_COMMIT}",
                  credentialsId: "${JOB_GCS_CREDENTIALS}",
                  pathPrefix: "${BASE_DIR}/build/distributions/",
                  pattern: "${BASE_DIR}/build/distributions/**/*",
                  sharedPublicly: true,
                  showInline: true)
              }
            }
          }
        }
        stage('APM Integration Tests') {
          agent { label 'linux && immutable' }
          options { skipDefaultCheckout() }
          when {
            beforeAgent true
            allOf {
              anyOf {
                changeRequest()
                expression { return !params.Run_As_Master_Branch }
              }
              expression { return params.its_ci }
              expression { return env.ONLY_DOCS == "false" }
            }
          }
          steps {
            withGithubNotify(context: 'APM Integration Tests') {
              script {
                def buildObject = build(job: env.ITS_PIPELINE, propagate: false, wait: true,
                      parameters: [string(name: 'INTEGRATION_TEST', value: 'All'),
                                  string(name: 'BUILD_OPTS', value: "--apm-server-build https://github.com/elastic/${env.REPO}@${env.GIT_BASE_COMMIT}")])
                copyArtifacts(projectName: env.ITS_PIPELINE, selector: specific(buildNumber: buildObject.number.toString()))
              }
            }
          }
          post {
            always {
              junit(testResults: "**/*-junit*.xml", allowEmptyResults: true, keepLongStdio: true)
            }
          }
        }
      }
    }
  }
  post {
    success {
      writeFile(file: 'beats-tester.properties',
                text: """\
                ## To be consumed by the beats-tester pipeline
                COMMIT=${env.GIT_BASE_COMMIT}
                APM_URL_BASE=https://storage.googleapis.com/${env.JOB_GCS_BUCKET}/commits/${env.GIT_BASE_COMMIT}
                VERSION=${env.APM_SERVER_VERSION}-SNAPSHOT""".stripIndent()) // stripIdent() requires '''/
      archiveArtifacts artifacts: 'beats-tester.properties'
    }
    cleanup {
      notifyBuildResult()
    }
  }
}

def golang(Closure body){
  def golangDocker
  retry(3) { // Retry in case there are any errors when building the docker images (to avoid temporary glitches)
    sleep randomNumber(min: 2, max: 5)
    golangDocker = docker.build('golang-mage', "--build-arg GO_VERSION=${GO_VERSION} -f  ${BASE_DIR}/.ci/docker/golang-mage/Dockerfile ${BASE_DIR}")
  }
  withEnv(["HOME=${WORKSPACE}", "GOPATH=${WORKSPACE}", "SHELL=/bin/bash"]) {
     golangDocker.inside('-v /usr/bin/docker:/usr/bin/docker -v /var/run/docker.sock:/var/run/docker.sock'){
       body()
     }
   }
}
