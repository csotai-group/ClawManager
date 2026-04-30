# Docker Push to Aliyun

## Description
Build and push ClawManager Docker image to Aliyun Container Registry.

## Registry
- **Address**: `crpi-p7zet5k9t1r8eu0j.cn-shenzhen.personal.cr.aliyuncs.com`
- **Repository**: `yangkin/clawmanager`
- **Tag**: `latest` (default)
- **Platform**: `linux/amd64`

## Usage

### Quick push (from project root)
```bash
./push-image.sh [tag]
```

### From backend directory
```bash
cd backend && make push-aliyun
```

### Manual steps
```bash
cd /Users/yangkai/workspace/ClawManager
docker build --platform linux/amd64 -t clawmanager:latest -f Dockerfile .
docker tag clawmanager:latest crpi-p7zet5k9t1r8eu0j.cn-shenzhen.personal.cr.aliyuncs.com/yangkin/clawmanager:latest
docker push crpi-p7zet5k9t1r8eu0j.cn-shenzhen.personal.cr.aliyuncs.com/yangkin/clawmanager:latest
```

## Notes
- Docker login credentials are persisted in `~/.docker/config.json`
- The registry is already authenticated; no extra login needed
- Image is built for `linux/amd64` by default
