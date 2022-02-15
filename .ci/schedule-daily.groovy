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
        updateBeatsBuilds(branches: ['main', '8.<minor>', '8.<next-minor>', '8.<next-patch>', '7.<minor>'])
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
  def branches = []
  // Expand macros and filter duplicated matches.
  args.branches.each { branch ->
    def branchName = getBranchName(branch)
    if (!branches.contains(branchName)) {
      branches << branchName
    }
  }
  branches.each { branch ->
    build(job: "apm-server/update-beats-mbp/${branch}", wait: false, propagate: false)
  }
}

def getBranchName(branch) {
  // special macro to look for the latest minor version
  if (branch.contains('8.<minor>')) {
   return bumpUtils.getMajorMinor(bumpUtils.getCurrentMinorReleaseFor8())
  }
  if (branch.contains('8.<next-minor>')) {
    return bumpUtils.getMajorMinor(bumpUtils.getNextMinorReleaseFor8())
  }
  // special macro to look for the latest minor version
  if (branch.contains('8.<next-patch>')) {
    return bumpUtils.getMajorMinor(bumpUtils.getNextPatchReleaseFor8())
  }
  if (branch.contains('7.<minor>')) {
    return bumpUtils.getMajorMinor(bumpUtils.getCurrentMinorReleaseFor7())
  }
  if (branch.contains('7.<next-minor>')) {
    return bumpUtils.getMajorMinor(bumpUtils.getNextMinorReleaseFor7())
  }
  return branch
}
