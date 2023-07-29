# crcsum
Simple CRC CLI tool

## Linux Alias

```
alias crc32='python crcsum.py'
```

## Env Setup

### Windows

https://docs.python.org/3/library/venv.html

From the root of your repository

```
python -m venv --prompt crc-dev-env .
Scripts\activate.bat
python -m pip install -r requirements.txt
```

The above is from a cmd.exe prompt. If you are using Powershell, then use the .ps1 script.

You can easily reactive the `venv` workspace with the command

```
Scripts\activate.bat
```

