# Delete and stop the service if it already exists.
if (Get-Service apm-server -ErrorAction SilentlyContinue) {
  $service = Get-WmiObject -Class Win32_Service -Filter "name='apm-server'"
  $service.StopService()
  Start-Sleep -s 1
  $service.delete()
}
