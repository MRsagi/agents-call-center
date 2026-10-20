# Inject agents-call-center / PAP2T working context for this repo.
# Used by sessionStart and beforeSubmitPrompt (reliable injection path).
$inputJson = [Console]::In.ReadToEnd()
$ctx = @'
agents-call-center hardware/SIP notes (project hook):
- Landline -> PAP2T FXS1; PAP2T LAN IP 192.168.10.75; PC USB-Ethernet 192.168.10.1
- Run: ACC_LINE_BACKEND=sip, ACC_SIP_PUBLIC_HOST=192.168.10.1, then acc serve
- After restarting acc, force PAP2T re-REGISTER (save Line 1 / bump Register Expires)
- PAP2T: Proxy 192.168.10.1, User/Auth 100, password acc, codec G711u, DTMF SIP INFO
- INVITE SDP must include Content-Type application/sdp; RTP ptime 30ms + keepalive
- Focus: --app target must be running before DTMF delivery; re-focus happens immediately before Type
- Test: .\acc.exe call --prompt "..." --app Notepad --digits 1 (open Notepad first)
- Phone notify hooks:
  - After each assistant message, stash text; mark multi-option decisions (1./2./3.)
  - On agent stop: call and speak what was done; press 1 to ack
  - If a decision was pending: speak the options, collect DTMF, inject "I choose option N" as follow-up so the agent continues
  - Risky shell: Cursor ask + ring
'@

# beforeSubmitPrompt wants continue; sessionStart ignores unknown fields safely.
$out = @{
  continue            = $true
  additional_context  = $ctx.Trim()
} | ConvertTo-Json -Compress
[Console]::Out.Write($out)
exit 0
