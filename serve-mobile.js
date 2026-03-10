#!/usr/bin/env node

/**
 * Simple HTTP Server for BETELITE Mobile Testing
 * Run with: node serve-mobile.js
 * 
 * Access mobile app at: http://localhost:8080/mobile/
 * Or from phone: http://YOUR_IP:8080/mobile/
 */

const http = require('http');
const fs = require('fs');
const path = require('path');
const os = require('os');
const url = require('url');

const PORT = process.env.PORT || 8080;
const MOBILE_DIR = path.join(__dirname, 'mobile');
const DOCS_DIR = path.join(__dirname, 'docs');

// MIME types
const mimeTypes = {
    '.html': 'text/html',
    '.js': 'application/javascript',
    '.json': 'application/json',
    '.css': 'text/css',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.gif': 'image/gif',
    '.svg': 'image/svg+xml',
    '.woff': 'font/woff',
    '.woff2': 'font/woff2',
    '.ttf': 'font/ttf',
    '.eot': 'application/vnd.ms-fontobject',
    '.mp4': 'video/mp4',
    '.webm': 'video/webm'
};

function getContentType(filePath) {
    const ext = path.extname(filePath).toLowerCase();
    return mimeTypes[ext] || 'application/octet-stream';
}

function getLocalIP() {
    const interfaces = os.networkInterfaces();
    for (const name of Object.keys(interfaces)) {
        for (const iface of interfaces[name]) {
            // Skip internal and non-IPv4 addresses
            if (iface.family === 'IPv4' && !iface.internal) {
                return iface.address;
            }
        }
    }
    return 'localhost';
}

const server = http.createServer((req, res) => {
    // Parse URL
    const parsedUrl = url.parse(req.url, true);
    let pathname = parsedUrl.pathname;

    // Default to index.html for root
    if (pathname === '/' || pathname === '') {
        pathname = '/docs/index.html';
    } else if (pathname.startsWith('/mobile')) {
        // Serve mobile app
        if (pathname === '/mobile' || pathname === '/mobile/') {
            pathname = '/mobile/index.html';
        }
    } else if (!path.extname(pathname)) {
        // Try index.html for directories
        pathname = path.join(pathname, 'index.html');
    }

    // Build file path
    let filePath = path.join(__dirname, pathname);

    // Security: Prevent directory traversal
    const realPath = path.resolve(filePath);
    if (!realPath.startsWith(path.resolve(__dirname))) {
        res.writeHead(403, { 'Content-Type': 'text/plain' });
        res.end('Forbidden');
        return;
    }

    // Read and serve file
    fs.readFile(filePath, (err, content) => {
        if (err) {
            // Try adding .html
            if (!filePath.endsWith('.html')) {
                return fs.readFile(filePath + '.html', (err, content) => {
                    if (err) {
                        res.writeHead(404, { 'Content-Type': 'text/html' });
                        res.end('<h1>404 Not Found</h1><p>' + pathname + '</p>');
                        return;
                    }
                    serveContent(filePath + '.html', content, res);
                });
            }

            res.writeHead(404, { 'Content-Type': 'text/html' });
            res.end('<h1>404 Not Found</h1><p>' + pathname + '</p>');
            return;
        }

        serveContent(filePath, content, res);
    });
});

function serveContent(filePath, content, res) {
    res.writeHead(200, {
        'Content-Type': getContentType(filePath),
        'Cache-Control': 'no-cache',
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
        'Access-Control-Allow-Headers': 'Content-Type, Authorization'
    });
    res.end(content);
}

// Handle OPTIONS requests for CORS
const originalCreateServer = http.createServer;
http.createServer = function(requestListener) {
    const server = originalCreateServer.call(this, (req, res) => {
        if (req.method === 'OPTIONS') {
            res.writeHead(200, {
                'Access-Control-Allow-Origin': '*',
                'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
                'Access-Control-Allow-Headers': 'Content-Type, Authorization'
            });
            res.end();
            return;
        }
        if (requestListener) requestListener(req, res);
    });
    return server;
};

const localIP = getLocalIP();

server.listen(PORT, '0.0.0.0', () => {
    console.log('\n╔════════════════════════════════════════╗');
    console.log('║  BETELITE Mobile Server Running        ║');
    console.log('╚════════════════════════════════════════╝\n');
    
    console.log(`📱 Mobile App:      http://localhost:${PORT}/mobile/`);
    console.log(`🌐 Landing Page:    http://localhost:${PORT}/`);
    console.log(`\n📡 From Your Phone (on same WiFi):`);
    console.log(`   http://${localIP}:${PORT}/mobile/\n`);
    
    console.log(`✓ Server running on port ${PORT}`);
    console.log(`✓ Press Ctrl+C to stop\n`);
});

process.on('SIGINT', () => {
    console.log('\n\n✓ Server stopped');
    process.exit(0);
});
