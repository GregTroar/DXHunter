# Parse token, host, and repo path from git remote URL
$remoteUrl = git remote get-url origin
if ($remoteUrl -match 'https://([^@]+)@([^/]+)/(.+?)\.git') {
    $token   = $Matches[1]
    $gitHost = $Matches[2]
    $repo    = $Matches[3]
} else {
    Write-Host "Cannot parse remote URL." -ForegroundColor Red; exit 1
}

git add .
$msg = Read-Host "Commit message"
if ($msg) { git commit -m $msg }

$ver = Read-Host "Version (ex: 2.47)"
if (-not $ver) { Write-Host "Aborted." -ForegroundColor Red; exit 1 }

(Get-Content Makefile) -replace 'VERSION = .*', "VERSION = $ver" | Set-Content Makefile
git add Makefile
git commit -m "chore: release v$ver"
git tag "v$ver"
git push
git push --tags

# Build Windows exe locally
Write-Host "Building DXHunter.exe v$ver..." -ForegroundColor Cyan
$env:CGO_ENABLED = '0'
go build -o DXHunter.exe -ldflags "-X main.version=$ver -H=windowsgui" .
if ($LASTEXITCODE -ne 0) { Write-Host "Build failed!" -ForegroundColor Red; exit 1 }

# Create Gitea release via API
$api     = "https://$gitHost/api/v1/repos/$repo"
$headers = @{ Authorization = "token $token"; 'Content-Type' = 'application/json' }
$payload = @{ tag_name = "v$ver"; name = "DXHunter v$ver"; body = "" } | ConvertTo-Json
$release = Invoke-RestMethod "$api/releases" -Method POST -Headers $headers -Body $payload

# Upload DXHunter.exe as release asset (curl.exe ships with Windows 10+)
$uploadUri = "https://$gitHost/api/v1/repos/$repo/releases/$($release.id)/assets?name=DXHunter.exe"
curl.exe -s -H "Authorization: token $token" -F "attachment=@DXHunter.exe" $uploadUri | Out-Null

Write-Host "Release v$ver published with DXHunter.exe!" -ForegroundColor Green
