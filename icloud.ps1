# Run from a local terminal so passwords and verification codes stay interactive.
& "$PSScriptRoot/.venv/Scripts/python.exe" "$PSScriptRoot/tools/reference/icloud.py" @args
exit $LASTEXITCODE
