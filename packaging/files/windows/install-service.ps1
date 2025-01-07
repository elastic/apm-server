# Delete and stop the service if it already exists.
if (Get-Service apm-server -ErrorAction SilentlyContinue) {
  $service = Get-WmiObject -Class Win32_Service -Filter "name='apm-server'"
  $service.StopService()
  Start-Sleep -s 1
  $service.delete()
}

$workdir = Split-Path $MyInvocation.MyCommand.Path

# Create the new service.
New-Service -name apm-server `
  -displayName apm-server`
  -binaryPathName "`"$workdir\apm-server.exe`" --environment=windows_service -c `"$workdir\apm-server.yml`" --path.home `"$workdir`" --path.data `"$env:PROGRAMFILES\apm-server`" --path.logs `"$env:PROGRAMFILES\apm-server\logs`" -E logging.files.redirect_stderr=true"

# Attempt to set the service to delayed start using sc config.
Try {
  Start-Process -FilePath sc.exe -ArgumentList 'config apm-server start= delayed-auto'
}
Catch { Write-Host -f red "An error occured setting the service to delayed start." }
