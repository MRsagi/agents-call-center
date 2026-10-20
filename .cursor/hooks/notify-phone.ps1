# Phone the human via acc when the agent finishes or needs a decision.
# Modes: stop | response | shell  (pass as: -Mode stop)
#
# Flow:
# - afterAgentResponse: stash the assistant text (needed for the spoken reply)
# - stop (completed): enqueue a call that speaks the response (does not block the agent);
#   if the human presses a digit, acc types it into Cursor; hangup with no digit = done
# - decisions: wait for DTMF and return followup_message so Cursor continues
# - shell (risky): ask in Cursor UI + ring
$ErrorActionPreference = 'Continue'
$Mode = 'response'
for ($i = 0; $i -lt $args.Count; $i++) {
  if ($args[$i] -eq '-Mode' -and ($i + 1) -lt $args.Count) {
    $Mode = [string]$args[$i + 1]
  }
}
if ($Mode -notin @('stop', 'response', 'shell')) { $Mode = 'response' }

$StateDir = Join-Path $env:TEMP 'acc-hooks'
if (-not (Test-Path $StateDir)) { New-Item -ItemType Directory -Path $StateDir | Out-Null }
$LastResponsePath = Join-Path $StateDir 'last-response.txt'
$PendingPath = Join-Path $StateDir 'pending-decision.json'
$RingStampPath = Join-Path $StateDir 'last-ring.txt'
# Digits pressed on the phone are typed into Cursor (hangup with none = no typing).
$FocusApp = 'Cursor'

$raw = [Console]::In.ReadToEnd()
if ([string]::IsNullOrWhiteSpace($raw)) { $raw = ($input | Out-String) }
$raw = $raw.Trim()
try { $evt = $raw | ConvertFrom-Json } catch { $evt = $null }

function Write-Json($obj) {
  if ($null -eq $obj) { '{}'; return }
  ($obj | ConvertTo-Json -Compress -Depth 8)
}

function Find-Acc {
  foreach ($c in @((Join-Path (Get-Location) 'acc.exe'), [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\acc.exe')))) {
    if (Test-Path $c) { return $c }
  }
  return $null
}

function Test-AccReady {
  $acc = Find-Acc
  if (-not $acc) { return $false }
  try {
    $s = & $acc status 2>$null | ConvertFrom-Json
    return ($s.health.ok -eq $true -and $s.health.line -eq 'up')
  } catch { return $false }
}

function Strip-Markdown([string]$text) {
  if ([string]::IsNullOrWhiteSpace($text)) { return '' }
  $t = $text
  $t = [regex]::Replace($t, '(?m)^#{1,6}\s*', '')
  $t = [regex]::Replace($t, '\[([^\]]+)\]\([^)]+\)', '$1')
  $t = [regex]::Replace($t, '[*`_~]', '')
  $t = [regex]::Replace($t, '\s+', ' ')
  return $t.Trim()
}

function Truncate-Speak([string]$text, [int]$max = 280) {
  $t = Strip-Markdown $text
  if ($t.Length -le $max) { return $t }
  return $t.Substring(0, $max - 3).Trim() + '...'
}

function Get-OptionCount([string]$text) {
  $n = 0
  foreach ($m in [regex]::Matches($text, '(?m)^\s*(?:#{1,6}\s*)?(?:Option\s+)?([1-9])[\.:\)]\s+\S')) {
    $v = [int]$m.Groups[1].Value
    if ($v -gt $n) { $n = $v }
  }
  if ($n -eq 0) {
    foreach ($m in [regex]::Matches($text, '(?i)\b(?:option|choice)\s*([1-9])\b')) {
      $v = [int]$m.Groups[1].Value
      if ($v -gt $n) { $n = $v }
    }
  }
  return $n
}

function Test-IsDecision([string]$text) {
  # Only numbered multi-choice (planning options). Open grill questions stay in Cursor.
  if ([string]::IsNullOrWhiteSpace($text) -or $text.Length -lt 40) { return $false }
  return (Get-OptionCount $text) -ge 2
}

function Build-DecisionPrompt([string]$text) {
  $opts = Get-OptionCount $text
  $lines = New-Object System.Collections.Generic.List[string]
  $lines.Add('Please choose an option.')
  foreach ($m in [regex]::Matches($text, '(?m)^\s*(?:#{1,6}\s*)?(?:Option\s+)?([1-9])[\.:\)]\s+(.+)$')) {
    $num = $m.Groups[1].Value
    $label = Strip-Markdown $m.Groups[2].Value
    if ($label.Length -gt 60) { $label = $label.Substring(0, 57).Trim() + '...' }
    $lines.Add("Press $num for $label")
  }
  if ($lines.Count -eq 1 -and $opts -ge 2) {
    $lines.Add("Press a number from 1 to $opts.")
  }
  $lines.Add('Press the digit now.')
  return Truncate-Speak (($lines -join ' ')) 320
}

function Build-DonePrompt([string]$text) {
  $summary = Truncate-Speak $text 300
  if ([string]::IsNullOrWhiteSpace($summary)) {
    return 'Agent finished with no text.'
  }
  return $summary
}

function Place-Call([string]$prompt, [int]$digits) {
  $acc = Find-Acc
  if (-not $acc) { return $null }
  if (-not (Test-AccReady)) { return $null }
  if ($digits -lt 1) { $digits = 1 }
  if ($digits -gt 9) { $digits = 9 }
  try {
    $out = & $acc @('call', '--prompt', $prompt, '--app', $FocusApp, '--digits', "$digits") 2>$null
    if ($LASTEXITCODE -ne 0) { return $null }
    $created = $out | ConvertFrom-Json
    if (-not $created.id) { return $null }
    Set-Content -Path $RingStampPath -Value ((Get-Date).ToString('o')) -Encoding ascii
    return [string]$created.id
  } catch { return $null }
}

function Wait-CallDigits([string]$id, [int]$timeoutSec = 120) {
  if ([string]::IsNullOrWhiteSpace($id)) { return $null }
  $base = if ($env:ACC_URL) { $env:ACC_URL.TrimEnd('/') } else { 'http://127.0.0.1:8741' }
  $deadline = (Get-Date).AddSeconds($timeoutSec)
  while ((Get-Date) -lt $deadline) {
    try {
      $c = Invoke-RestMethod -Uri "$base/v1/calls/$id" -TimeoutSec 5
      if ($c.status -in @('completed', 'failed', 'voicemail')) {
        if ($c.status -eq 'completed' -and $c.digits) { return [string]$c.digits }
        return $null
      }
    } catch {}
    Start-Sleep -Seconds 2
  }
  return $null
}

function Save-LastResponse([string]$text) {
  Set-Content -Path $LastResponsePath -Value $text -Encoding utf8
}

function Load-LastResponse {
  if (Test-Path $LastResponsePath) {
    return Get-Content -Path $LastResponsePath -Raw -Encoding utf8
  }
  return ''
}

switch ($Mode) {
  'response' {
    # Stash only — the call happens on stop when the agent turn is finished.
    $text = [string]$evt.text
    Save-LastResponse $text
    if (Test-IsDecision $text) {
      @{
        kind    = 'decision'
        options = (Get-OptionCount $text)
        savedAt = (Get-Date).ToString('o')
      } | ConvertTo-Json | Set-Content -Path $PendingPath -Encoding utf8
    } else {
      Remove-Item $PendingPath -Force -ErrorAction SilentlyContinue
    }
    Write-Json @{}
    exit 0
  }

  'stop' {
    $status = [string]$evt.status
    if ($status -ne 'completed') {
      Write-Json @{}
      exit 0
    }

    $last = Load-LastResponse
    $pending = $null
    if (Test-Path $PendingPath) {
      try { $pending = Get-Content $PendingPath -Raw | ConvertFrom-Json } catch { $pending = $null }
    }

    if ($pending -and $pending.kind -eq 'decision') {
      $prompt = Build-DecisionPrompt $last
      $id = Place-Call $prompt 1
      Remove-Item $PendingPath -Force -ErrorAction SilentlyContinue
      $digits = Wait-CallDigits $id 150
      if ($digits) {
        Write-Json @{
          followup_message = "I choose option $digits (via phone DTMF)."
        }
        exit 0
      }
      Write-Json @{}
      exit 0
    }

    # Agent finished: speak the response; do not block waiting for an ack.
    # Digit (if any) is typed by acc into Cursor; hangup with no digit ends the call.
    $prompt = Build-DonePrompt $last
    $null = Place-Call $prompt 1
    Remove-Item $PendingPath -Force -ErrorAction SilentlyContinue
    Write-Json @{}
    exit 0
  }

  'shell' {
    # Keep agent_message short — embedding the full shell command can break JSON parsing.
    $rang = $false
    try {
      $id = Place-Call 'Shell command needs your approval in Cursor.' 1
      $rang = [bool]$id
    } catch {}
    $msg = if ($rang) {
      'Review this shell command. Your phone should ring.'
    } else {
      'Review this shell command. (Phone notify skipped — is acc serve up with line up?)'
    }
    Write-Json @{
      permission    = 'ask'
      user_message  = $msg
      agent_message = 'Hook requested approval for a matched shell command.'
    }
    exit 0
  }
}

Write-Json @{}
exit 0
