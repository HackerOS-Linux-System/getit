# getit
Download directories from github/gitlab.

## Usage

```
getit file <url> [-o NAME]      download a single file (like curl/wget)
getit repo <url.git> -clone     clone a repository
getit repo <url.git> -push      push to a remote
getit dir  <github-url>         download a folder from GitHub
```

Examples:

```
getit dir https://github.com/user/repo/tree/main/src
getit dir github.com/user/repo
```

## Build

Requires H# (`hacker unpack h#` and `hacker unpack h#-utils`) and `bit`.

```
bit build --release
# or directly:
h# compile src/main.h# --release -o getit
```
