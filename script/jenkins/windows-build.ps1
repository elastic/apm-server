function Exec {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [scriptblock]$cmd,
        [string]$errorMessage = ($msgs.error_bad_command -f $cmd)
    )

    try {
        $global:lastexitcode = 0
        & $cmd 2>&1 | %{ "$_" }
        if ($lastexitcode -ne 0) {
            throw $errorMessage
        }
    }
    catch [Exception] {
        throw $_
    }
}

# Setup Go.
$env:GOPATH = $env:WORKSPACE
$env:PATH = "$env:GOPATH\bin;C:\tools\mingw64\bin;$env:PATH"
& gvm --format=powershell $(Get-Content .go-version) | Invoke-Expression

# Write cached magefile binaries to workspace to ensure
# each run starts from a clean slate.
$env:MAGEFILE_CACHE = "$env:WORKSPACE\.magefile"

# Configure testing parameters.
$env:TEST_COVERAGE = "true"
$env:RACE_DETECTOR = "true"

# Install mage.
exec { go install github.com/magefile/mage }

echo "Fetching testing dependencies"
# TODO (elastic/beats#5050): Use a vendored copy of this.
exec { go get github.com/docker/libcompose }

if (Test-Path "build") { Remove-Item -Recurse -Force build }
New-Item -ItemType directory -Path build\coverage | Out-Null
New-Item -ItemType directory -Path build\system-tests | Out-Null
New-Item -ItemType directory -Path build\system-tests\run | Out-Null

choco install python -y -r --no-progress --version 3.8.1.20200110
refreshenv
$env:PATH = "C:\Python38;C:\Python38\Scripts;$env:PATH"
$env:PYTHON_ENV = "$env:TEMP\python-env"
python --version

echo "Building fields.yml"
exec { mage fields }

echo "Building $env:beat"
exec { mage build } "Build FAILURE"
