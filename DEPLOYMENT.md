# DEPLOYMENT GUIDE

## PRE-DEPLOYMENT CHECKLIST

### Security
- [ ] All default credentials changed
- [ ] API keys generated for all services
- [ ] SSL/TLS certificate installed
- [ ] Firewall rules configured
- [ ] Database backed up
- [ ] Code reviewed for secrets

### Infrastructure
- [ ] PHP 8.2+ installed
- [ ] MySQL 8.0+ installed
- [ ] Apache/Nginx configured
- [ ] Composer dependencies installed
- [ ] Disk space >= 50GB
- [ ] RAM >= 4GB
- [ ] CPU >= 2 cores

### Configuration
- [ ] .env configured for production
- [ ] Database created and populated
- [ ] Logs directory writable
- [ ] Uploads directory writable
- [ ] Mail service configured
- [ ] Payment gateway credentials set

---

## STEP-BY-STEP DEPLOYMENT

### 1. Prepare Server

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install PHP 8.2
sudo apt install -y php8.2 php8.2-cli php8.2-fpm
sudo apt install -y php8.2-pdo php8.2-mysql php8.2-curl
sudo apt install -y php8.2-mbstring php8.2-xml

# Install MySQL
sudo apt install -y mysql-server mysql-client

# Install Nginx (or Apache)
sudo apt install -y nginx

# Install Composer
curl -sS https://getcomposer.org/installer | sudo php -- --install-dir=/usr/local/bin
```

### 2. Clone Application

```bash
# Navigate to web root
cd /var/www

# Clone repository
git clone https://github.com/betelite/betelite.git

# Set ownership
sudo chown -R www-data:www-data betelite

# Set permissions
chmod 750 betelite
chmod 700 betelite/logs
chmod 700 betelite/uploads
chmod 640 betelite/config.php
chmod 640 betelite/.env
```

### 3. Configure Environment

```bash
# Create .env from template
cp betelite/.env.example betelite/.env

# Edit configuration
nano betelite/.env

# Critical settings:
# DB_HOST=localhost
# DB_USER=betelite_user
# DB_PASSWORD=STRONG_PASSWORD_HERE
# APP_ENV=production
# JWT_SECRET=LONG_RANDOM_SECRET_HERE
# PAYSTACK_SECRET_KEY=pk_live_xxx
```

### 4. Setup Database

```bash
# Connect to MySQL
mysql -u root -p

# Create database and user
CREATE DATABASE betelite_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'betelite_user'@'localhost' IDENTIFIED BY 'STRONG_PASSWORD';
GRANT ALL PRIVILEGES ON betelite_db.* TO 'betelite_user'@'localhost';
FLUSH PRIVILEGES;
EXIT;

# Import schema
mysql -u betelite_user -p betelite_db < betelite/database/schema.sql

# Import seed data
mysql -u betelite_user -p betelite_db < betelite/database/seeds.sql
```

### 5. Configure Web Server

#### For Nginx:

```nginx
# /etc/nginx/sites-available/betelite
server {
    listen 443 ssl http2;
    server_name api.betelite.example.com;

    # SSL certificates
    ssl_certificate /etc/letsencrypt/live/betelite.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/betelite.example.com/privkey.pem;
    
    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # Root
    root /var/www/betelite;
    index index.php;

    # PHP processing
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
    }

    # Deny access to sensitive files
    location ~ /\. {
        deny all;
        access_log off;
        log_not_found off;
    }

    location ~ ~$ {
        deny all;
        access_log off;
        log_not_found off;
    }

    # Logging
    access_log /var/log/nginx/betelite-access.log;
    error_log /var/log/nginx/betelite-error.log;
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name api.betelite.example.com;
    return 301 https://$server_name$request_uri;
}
```

#### For Apache:

```apache
# /etc/apache2/sites-available/betelite.conf
<VirtualHost *:443>
    ServerName api.betelite.example.com
    DocumentRoot /var/www/betelite

    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/betelite.example.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/betelite.example.com/privkey.pem

    # Security Headers
    Header set Strict-Transport-Security "max-age=31536000"
    Header set X-Frame-Options "DENY"
    Header set X-Content-Type-Options "nosniff"

    # Enable mod_rewrite
    <Directory /var/www/betelite>
        Options -Indexes
        AllowOverride All
        Require all granted
    </Directory>

    # Logging
    CustomLog ${APACHE_LOG_DIR}/betelite-access.log combined
    ErrorLog ${APACHE_LOG_DIR}/betelite-error.log
</VirtualHost>

<VirtualHost *:80>
    ServerName api.betelite.example.com
    Redirect permanent / https://api.betelite.example.com/
</VirtualHost>
```

### 6. Enable SSL

```bash
# Install Certbot
sudo apt install -y certbot python3-certbot-nginx

# Generate certificate
sudo certbot certonly --nginx -d api.betelite.example.com

# Setup auto-renewal
sudo certbot renew --dry-run
```

### 7. Configure PHP

```ini
# /etc/php/8.2/fpm/php.ini
upload_max_filesize = 50M
post_max_size = 50M
max_execution_time = 300
memory_limit = 256M
default_timezone = UTC

# Security
expose_php = Off
display_errors = Off
log_errors = On
error_log = /var/log/php-errors.log
```

### 8. Setup Logging & Rotation

```bash
# Create logrotate config
sudo tee /etc/logrotate.d/betelite << EOF
/var/www/betelite/logs/*.log {
    daily
    rotate 90
    compress
    delaycompress
    notifempty
    create 0640 www-data www-data
    sharedscripts
}
EOF

# Apply config
sudo logrotate -f /etc/logrotate.d/betelite
```

### 9. Start Services

```bash
# Nginx
sudo systemctl restart nginx
sudo systemctl enable nginx

# PHP-FPM
sudo systemctl restart php8.2-fpm
sudo systemctl enable php8.2-fpm

# MySQL
sudo systemctl restart mysql
sudo systemctl enable mysql

# Verify
sudo systemctl status nginx
sudo systemctl status php8.2-fpm
sudo systemctl status mysql
```

### 10. Test Installation

```bash
# Health check
curl -k https://api.betelite.example.com/api/health

# Should return:
# {"status":"ok","version":"1.0.0","timestamp":"2026-01-13T10:30:00Z"}

# Check logs
tail -f /var/www/betelite/logs/betelite.log
tail -f /var/www/betelite/logs/audit.log
```

---

## PRODUCTION HARDENING

### Firewall Configuration

```bash
# UFW (Ubuntu)
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp      # SSH
sudo ufw allow 80/tcp      # HTTP
sudo ufw allow 443/tcp     # HTTPS
sudo ufw allow 3306/tcp from 127.0.0.1  # MySQL local only
sudo ufw enable
```

### MySQL Security

```bash
# Secure MySQL
mysql_secure_installation

# Remove test database
mysql -u root -p -e "DROP DATABASE test;"

# Verify user permissions
mysql -u root -p -e "SHOW GRANTS FOR 'betelite_user'@'localhost';"
```

### PHP Security

```bash
# Disable dangerous functions
disable_functions = exec,passthru,shell_exec,system,proc_open,popen,curl_exec,curl_multi_exec,parse_ini_file,show_source

# Restrict file uploads
open_basedir = /var/www/betelite:/tmp

# Disable external URLs
allow_url_fopen = Off
```

### Regular Backups

```bash
#!/bin/bash
# backup.sh - Daily backup script

BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d_%H%M%S)

# Database backup
mysqldump -u betelite_user -p'STRONG_PASSWORD' betelite_db | gzip > $BACKUP_DIR/db_$DATE.sql.gz

# Code backup
tar -czf $BACKUP_DIR/code_$DATE.tar.gz /var/www/betelite

# Upload to S3
aws s3 sync $BACKUP_DIR s3://betelite-backups/ --delete

# Cleanup old backups (keep 30 days)
find $BACKUP_DIR -type f -mtime +30 -delete

echo "[$(date)] Backup completed" >> /var/log/betelite-backup.log
```

Schedule with cron:
```bash
0 2 * * * /usr/local/bin/backup.sh
```

---

## MONITORING

### Application Health Check

```bash
#!/bin/bash
# health_check.sh

ENDPOINT="https://api.betelite.example.com/api/health"

response=$(curl -s -w "\n%{http_code}" "$ENDPOINT")
http_code=$(echo "$response" | tail -n 1)

if [ "$http_code" -eq 200 ]; then
    echo "✓ Health check OK"
    exit 0
else
    echo "✗ Health check failed: $http_code"
    # Alert admin
    exit 1
fi
```

### Monitor Key Metrics

```bash
# Active connections
netstat -an | grep ESTABLISHED | wc -l

# Disk usage
df -h /var/www/betelite

# Memory usage
free -h

# MySQL status
mysqladmin -u betelite_user -p status

# Log size
du -sh /var/www/betelite/logs/
```

---

## UPGRADE PROCEDURE

### Before Upgrade

```bash
# Backup everything
mysqldump -u betelite_user -p betelite_db > /backups/pre-upgrade.sql
tar -czf /backups/code-pre-upgrade.tar.gz /var/www/betelite
```

### Upgrade Steps

```bash
# 1. Stop services
sudo systemctl stop php8.2-fpm

# 2. Backup current code
cp -r /var/www/betelite /var/www/betelite.backup

# 3. Download new version
cd /var/www/betelite
git fetch origin
git checkout vX.X.X

# 4. Run migrations if needed
mysql -u betelite_user -p betelite_db < database/migrations.sql

# 5. Clear caches
rm -rf logs/*

# 6. Restart services
sudo systemctl start php8.2-fpm

# 7. Verify
curl https://api.betelite.example.com/api/health
```

### Rollback on Failure

```bash
# Restore from backup
rm -rf /var/www/betelite
cp -r /var/www/betelite.backup /var/www/betelite

# Restore database
mysql -u betelite_user -p betelite_db < /backups/pre-upgrade.sql

# Restart services
sudo systemctl restart php8.2-fpm
```

---

**Last Updated**: 2026-01-13
**Status**: Production Ready
