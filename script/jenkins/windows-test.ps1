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
& go install github.com/elastic/apm-server/vendor/github.com/magefile/mage 2>&1 | %{ "$_" }

echo "Fetching testing dependencies"
# TODO (elastic/beats#5050): Use a vendored copy of this.
& go get github.com/docker/libcompose 2>&1 | %{ "$_" }

if (Test-Path "build") { Remove-Item -Recurse -Force build }
New-Item -ItemType directory -Path build\coverage | Out-Null
New-Item -ItemType directory -Path build\system-tests | Out-Null
New-Item -ItemType directory -Path build\system-tests\run | Out-Null

echo "Building fields.yml"
& mage fields 2>&1 | %{ "$_" }

echo "Building $env:beat"
& mage build 2>&1 | %{ "$_" }

echo "Unit testing $env:beat"
& mage goTestUnit 2>&1 | %{ "$_" }

echo "System testing $env:beat"
# Get a CSV list of package names.
$packages = $(go list ./... | select-string -Pattern "/vendor/" -NotMatch | select-string -Pattern "/scripts/cmd/" -NotMatch)
$packages = ($packages|group|Select -ExpandProperty Name) -join ","
& go test -race -c -cover -covermode=atomic -coverpkg $packages 2>&1 | %{ "$_" }
Set-Location -Path tests/system
& nosetests --with-timer --with-xunit --xunit-file=../../build/TEST-system.xml 2>&1 | %{ "$_" }
