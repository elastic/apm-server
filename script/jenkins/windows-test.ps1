$ErrorActionPreference = "Stop"
trap { "Error trapped: $_"; break }

# Setup Go.
$env:GOPATH = $env:WORKSPACE
$env:GO111MODULE = "off"
$env:PATH = "$env:GOPATH\bin;C:\tools\mingw64\bin;$env:PATH"
& gvm --format=powershell $(Get-Content .go-version) | Invoke-Expression

# Write cached magefile binaries to workspace to ensure
# each run starts from a clean slate.
$env:MAGEFILE_CACHE = "$env:WORKSPACE\.magefile"

# Configure testing parameters.
$env:TEST_COVERAGE = "true"
$env:RACE_DETECTOR = "true"

# Install mage from vendor.
& go install github.com/elastic/apm-server/vendor/github.com/magefile/mage

echo "Fetching testing dependencies"
# TODO (elastic/beats#5050): Use a vendored copy of this.
& go get github.com/docker/libcompose

if (Test-Path "build") { Remove-Item -Recurse -Force build }
New-Item -ItemType directory -Path build\coverage | Out-Null
New-Item -ItemType directory -Path build\system-tests | Out-Null
New-Item -ItemType directory -Path build\system-tests\run | Out-Null

echo "Building fields.yml"
& mage fields

echo "Building $env:beat"
& mage build

echo "Unit testing $env:beat"
& mage goTestUnit

echo "System testing $env:beat"
# Get a CSV list of package names.
$packages = $(go list ./... | select-string -Pattern "/vendor/" -NotMatch | select-string -Pattern "/scripts/cmd/" -NotMatch)
$packages = ($packages|group|Select -ExpandProperty Name) -join ","
& go test -race -c -cover -covermode=atomic -coverpkg $packages
Set-Location -Path tests/system
& nosetests --with-timer --with-xunit --xunit-file=../../build/TEST-system.xml
