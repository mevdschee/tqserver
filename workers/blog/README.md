# Blog Worker (PHP Example)

This is a test worker to demonstrate TQServer's PHP-FPM alternative functionality.

## Structure

```
workers/blog/
├── config/
│   └── worker.yaml      # Worker configuration
├── public/
│   ├── hello.php        # Simple hello world
│   └── info.php         # PHP info page
└── README.md            # This file
```

## Quick Start

### 1. Verify PHP is installed

```bash
php-cgi -v
```

### 2. Test with php-cgi directly

```bash
cd workers/blog/public
php-cgi -b /tmp/tqserver-blog.sock
```

### 3. Configure Nginx (for testing)

Create `/etc/nginx/sites-available/tqserver-blog`:

```nginx
server {
    listen 8080;
    server_name localhost;
    
    root /home/maurits/projects/tqserver/workers/blog/public;
    index index.php hello.php;
    
    location / {
        try_files $uri $uri/ =404;
    }
    
    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_pass unix:/tmp/tqserver-blog.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        fastcgi_param PATH_INFO $fastcgi_path_info;
    }
}
```

Enable and restart Nginx:

```bash
sudo ln -s /etc/nginx/sites-available/tqserver-blog /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 4. Test the setup

```bash
# Test hello.php
curl http://localhost:8080/hello.php

# Test info.php
curl http://localhost:8080/info.php
```

## Development Roadmap

### Phase 1: Basic FastCGI ✅
- [x] FastCGI protocol implementation
- [x] Connection handling
- [x] Protocol tests
- [ ] Integration with TQServer routing

### Phase 2: PHP-CGI Integration
- [ ] Spawn php-cgi workers
- [ ] Configure via CLI flags
- [ ] Request proxying
- [ ] Error handling

### Phase 3: Pool Management
- [ ] Static pool manager
- [ ] Dynamic pool manager
- [ ] Ondemand pool manager
- [ ] Health monitoring

### Phase 4: Advanced Features
- [ ] Hot reload support
- [ ] Multiple PHP versions
- [ ] Slow request logging
- [ ] Performance metrics

## Configuration Notes

The `worker.yaml` file demonstrates TQServer's approach:

1. **No PHP-FPM config files**: All pool/process management is in TQServer's YAML
2. **Optional php.ini**: Can use existing ini files as base configuration
3. **CLI overrides**: Individual settings via `-d` flags to php-cgi
4. **Flexible pools**: Different configs per route/worker

## Testing Hello World

Once Phase 2 is complete, you'll be able to:

```bash
# Start TQServer
./server/bin/tqserver

# TQServer will:
# 1. Read workers/blog/config/worker.yaml
# 2. Spawn 2 php-cgi workers (static pool)
# 3. Start FastCGI server on /tmp/tqserver-blog.sock
# 4. Route requests from Nginx to PHP workers
```

Then visit:
- http://localhost:8080/hello.php - See "Hello from TQServer!"
- http://localhost:8080/info.php - See PHP configuration

## Next Steps

1. Complete FastCGI server integration with TQServer
2. Implement PHP-CGI process spawning
3. Test with Nginx forwarding
4. Add process pool management
5. Implement health checks and auto-recovery
