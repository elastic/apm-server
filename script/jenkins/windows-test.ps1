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

# Setup Python.
exec { choco install python2 -y -r --force --no-progress --version 2.7.17 }
refreshenv
$env:PATH = "C:\Python27;C:\Python27\Scripts;$env:PATH"
$env:PYTHON_ENV = "$env:TEMP\python-env"
exec { python --version }
exec { pip install virtualenv }

# Configure testing parameters.
$env:TEST_COVERAGE = "true"
$env:RACE_DETECTOR = "true"

# Install mage from vendor.
exec { go install github.com/elastic/apm-server/vendor/github.com/magefile/mage }

echo "Fetching testing dependencies"
# TODO (elastic/beats#5050): Use a vendored copy of this.
exec { go get github.com/docker/libcompose }

if (Test-Path "build") { Remove-Item -Recurse -Force build }
New-Item -ItemType directory -Path build\coverage | Out-Null
New-Item -ItemType directory -Path build\system-tests | Out-Null
New-Item -ItemType directory -Path build\system-tests\run | Out-Null

echo "Building fields.yml"
exec { mage fields }

echo "Building $env:beat"
exec { mage build } "Build FAILURE"

echo "Unit testing $env:beat"
exec { mage goTestUnit }

echo "System testing $env:beat"
# Get a CSV list of package names.
$packages = $(go list ./... | select-string -Pattern "/vendor/" -NotMatch | select-string -Pattern "/scripts/cmd/" -NotMatch)
$packages = ($packages|group|Select -ExpandProperty Name) -join ","
exec { go test -race -c -cover -covermode=atomic -coverpkg $packages } "go test FAILURE"

echo "Running python tests"
exec { mage pythonUnitTest } "System test FAILURE"
