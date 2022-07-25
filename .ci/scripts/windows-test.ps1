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

$env:GOPATH = $env:WORKSPACE
$env:PATH = "$env:GOPATH\bin;C:\tools\mingw64\bin;$env:PATH"
& gvm --format=powershell $(Get-Content .go-version) | Invoke-Expression
exec { go test -v ./... }
