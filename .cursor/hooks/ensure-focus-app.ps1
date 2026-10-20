# Before `acc call`, ensure the --app focus target is running so Type does not
# land in the wrong window (e.g. the terminal).
$raw = [Console]::In.ReadToEnd()
try {
  $evt = $raw | ConvertFrom-Json
} catch {
  Write-Output '{"permission":"allow"}'
  exit 0
}

$cmd = [string]$evt.command
if ($cmd -notmatch '(?i)acc(\.exe)?\s+call') {
  Write-Output '{"permission":"allow"}'
  exit 0
}

$app = $null
if ($cmd -match '(?i)--app\s+"([^"]+)"') {
  $app = $Matches[1]
} elseif ($cmd -match '(?i)--app\s+(\S+)') {
  $app = $Matches[1]
}

if ([string]::IsNullOrWhiteSpace($app)) {
  Write-Output '{"permission":"allow"}'
  exit 0
}

$started = $false
$msg = $null
switch -Regex ($app) {
  '^(?i)notepad(\.exe)?$' {
    $proc = Get-Process -Name notepad -ErrorAction SilentlyContinue
    if (-not $proc) {
      Start-Process notepad.exe
      Start-Sleep -Milliseconds 400
      $started = $true
      $msg = 'Opened Notepad for acc call focus target.'
    }
  }
  '^(?i)calc(ulator)?(\.exe)?$' {
    $proc = Get-Process -Name CalculatorApp, win32calc, calc -ErrorAction SilentlyContinue
    if (-not $proc) {
      Start-Process calc.exe
      Start-Sleep -Milliseconds 400
      $started = $true
      $msg = 'Opened Calculator for acc call focus target.'
    }
  }
  default {
    # Unknown app: do not block; agent/user must open it.
    $msg = "Focus app '$app' is not auto-launched by the hook; ensure it is open before DTMF."
  }
}

$result = @{ permission = 'allow' }
if ($msg) {
  $result.agent_message = $msg
  if ($started) {
    $result.user_message = $msg
  }
}
$result | ConvertTo-Json -Compress
exit 0
