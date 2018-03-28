function Exec
{
    param(
        [Parameter(Position=0,Mandatory=1)][scriptblock]$cmd,
        [Parameter(Position=1,Mandatory=0)][string]$errorMessage = ($msgs.error_bad_command -f $cmd)
    )

    & $cmd
    if ($LastExitCode -ne 0) {
        Write-Error $errorMessage
        exit $LastExitCode
    }
}

# Setup Go.
$env:GOPATH = $env:WORKSPACE
$env:PATH = "$env:GOPATH\bin;C:\tools\mingw64\bin;$env:PATH"
if (Test-Path -PathType leaf _beats\.go-version) {
    & gvm --format=powershell $(Get-Content _beats\.go-version) | Invoke-Expression
} else {
    & gvm --format=powershell 1.9.4 | Invoke-Expression
}

if (Test-Path "build") { Remove-Item -Recurse -Force build }
New-Item -ItemType directory -Path build\coverage | Out-Null
New-Item -ItemType directory -Path build\system-tests | Out-Null
New-Item -ItemType directory -Path build\system-tests\run | Out-Null

exec { go get -u github.com/jstemmer/go-junit-report }

echo "Building $env:beat"
exec { go build } "Build FAILURE"


cp .\_meta\fields.common.yml .\_meta\fields.generated.yml
cat processor\*\_meta\fields.yml | Out-File -append -encoding UTF8 -filepath .\_meta\fields.generated.yml
cp .\_meta\fields.generated.yml .\fields.yml



echo "Unit testing $env:beat"
go test -v $(go list ./... | select-string -Pattern "vendor" -NotMatch) 2>&1 | Out-File -encoding UTF8 build/TEST-go-unit.out
exec { Get-Content build/TEST-go-unit.out | go-junit-report.exe -set-exit-code | Out-File -encoding UTF8 build/TEST-go-unit.xml } "Unit test FAILURE"

echo "System testing $env:beat"
exec { go test -race -c -cover -covermode=atomic -coverpkg ./... }
exec { cd tests/system }
exec { nosetests --with-timer --with-xunit --xunit-file=../../build/TEST-system.xml } "System test FAILURE"
