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
    cron('H H(2-3) * * 1-5')
  }
  stages {
    stage('Nighly update Beats builds') {
      steps {
        runWindowsBuilds(branches: ['main', '8.<minor>', '8.<next-patch>', '7.<minor>'])
        runSmokeTestsOs(branches: ['main', '8.<minor>', '8.<next-patch>'])
        runSmokeTestsEss(branches: ['main'])
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
  }
}

def runWindowsBuilds(Map args = [:]) {
  def branches = getBranchesFromAliases(aliases: args.branches)
  branches.each { branch ->
    build(job: "apm-server/apm-server-mbp/${branch}",
          parameters: [
            booleanParam(name: 'windows_ci', value: true)
          ],
          wait: false, propagate: false)
  }
}

def runSmokeTestsEss(Map args = [:]) {
  def branches = getBranchesFromAliases(aliases: args.branches)
  branches.each { branch ->
    build(job: "apm-server/smoke-tests-ess-mbp/${branch}", wait: false, propagate: false)
  }
}

def runSmokeTestsOs(Map args = [:]) {
  def branches = getBranchesFromAliases(aliases: args.branches)
  branches.each { branch ->
    build(job: "apm-server/smoke-tests-os-mbp/${branch}", wait: false, propagate: false)
  }
}
