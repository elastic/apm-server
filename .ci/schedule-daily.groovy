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
        updateBeatsBuilds(branches: ['main', '8.<minor>', '8.<next-patch>', '7.<minor>'])
        runWindowsBuilds(branches: ['main', '8.<minor>', '8.<next-patch>', '7.<minor>'])
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
  }
}

def updateBeatsBuilds(Map args = [:]) {
  def branches = getBranchesFromAliases(aliases: args.branches)
  branches.each { branch ->
    build(job: "apm-server/update-beats-mbp/${branch}", wait: false, propagate: false)
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
