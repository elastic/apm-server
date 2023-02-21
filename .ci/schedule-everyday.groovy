@Library('apm@current') _

pipeline {
  agent none
  environment {
    NOTIFY_TO = credentials('notify-to')
    PIPELINE_LOG_LEVEL = 'INFO'
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
  }
  triggers {
    cron('H H(17-18) * * *')
  }
  stages {
    stage('Nighly benchmarks') {
      steps {
        runBenchmarks(branches: ['main'])
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
  }
}

def runBenchmarks(Map args = [:]) {
  def branches = getBranchesFromAliases(aliases: args.branches)
  branches.each { branch ->
    build(job: "apm-server/benchmarks/${branch}", wait: false, propagate: false)
  }
}
