/*
Copyright 2025 Kube-nova By YanShicheng.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package builder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
)

// buildNginxConfigMap 构建 Nginx ConfigMap
func buildNginxConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	nginxConf := buildNginxConf()
	defaultConf := buildDefaultConf(kn, namespace)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend-nginx-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"nginx.conf":   nginxConf,
			"default.conf": defaultConf,
		},
	}
}

// buildNginxConf 构建 nginx.conf
func buildNginxConf() string {
	return `# Nginx main configuration file
worker_processes auto;
worker_rlimit_nofile 65535;

error_log /var/log/nginx/error.log warn;

events {
    worker_connections 4096;
    use epoll;
    multi_accept on;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # Logging format
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for" '
                    'rt=$request_time uct="$upstream_connect_time" '
                    'uht="$upstream_header_time" urt="$upstream_response_time"';

    log_format json escape=json '{'
                    '"time_local":"$time_local",'
                    '"remote_addr":"$remote_addr",'
                    '"remote_user":"$remote_user",'
                    '"request":"$request",'
                    '"status":"$status",'
                    '"body_bytes_sent":"$body_bytes_sent",'
                    '"request_time":"$request_time",'
                    '"http_referrer":"$http_referer",'
                    '"http_user_agent":"$http_user_agent",'
                    '"http_x_forwarded_for":"$http_x_forwarded_for",'
                    '"upstream_addr":"$upstream_addr",'
                    '"upstream_status":"$upstream_status",'
                    '"upstream_response_time":"$upstream_response_time",'
                    '"upstream_connect_time":"$upstream_connect_time",'
                    '"upstream_header_time":"$upstream_header_time"'
                    '}';

    access_log /var/log/nginx/access.log main;

    # Performance optimization
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    keepalive_requests 100;
    types_hash_max_size 2048;
    server_tokens off;

    # Buffer settings
    client_body_buffer_size 128k;
    client_max_body_size 1024m;
    client_header_buffer_size 1k;
    large_client_header_buffers 4 16k;
    output_buffers 1 32k;
    postpone_output 1460;

    # Timeout settings
    client_header_timeout 15s;
    client_body_timeout 15s;
    send_timeout 15s;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript
               application/json application/javascript application/xml+rss
               application/rss+xml font/truetype font/opentype
               application/vnd.ms-fontobject image/svg+xml;
    gzip_disable "msie6";
    gzip_min_length 1000;
    gzip_buffers 16 8k;

    # Proxy cache settings
    proxy_cache_path /var/cache/nginx/proxy_cache levels=1:2 keys_zone=api_cache:10m max_size=100m inactive=60m use_temp_path=off;
    proxy_cache_key "$scheme$request_method$host$request_uri";
    proxy_cache_valid 200 302 10m;
    proxy_cache_valid 404 1m;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=general:10m rate=100r/s;
    limit_req_zone $binary_remote_addr zone=api:10m rate=50r/s;
    limit_conn_zone $binary_remote_addr zone=addr:10m;

    # Include virtual host configs
    include /etc/nginx/conf.d/*.conf;
}
`
}

// buildDefaultConf 构建 default.conf
func buildDefaultConf(kn *kubenovav1.KubeNova, namespace string) string {
	config := `# Upstream definitions
upstream portal_api {
    least_conn;
    server portal-api:8810 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream manager_api {
    least_conn;
    server manager-api:8811 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream workload_api {
    least_conn;
    server workload-api:8812 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream console_api {
    least_conn;
    server console-api:8818 max_fails=3 fail_timeout=30s;
    keepalive 32;
}
`

	if kn.IsMinIOProxyEnabled() {
		storageEndpoint := kn.Spec.Storage.Endpoint

		upstreamConfig := fmt.Sprintf("server %s max_fails=3 fail_timeout=30s;", storageEndpoint)

		config += fmt.Sprintf(`
# MinIO upstream - 代理到实际 MinIO 地址
upstream minio_backend {
    least_conn;
    %s
    keepalive 32;
}
`, upstreamConfig)
	}

	// HTTP 服务器配置
	config += buildHTTPServerBlock(kn)

	// HTTPS 服务器配置（仅 NodePort + HTTPS 模式）
	if kn.Spec.Web.ExposeType == "nodeport" &&
		kn.Spec.Web.NodePort != nil &&
		kn.Spec.Web.NodePort.HTTPS != nil &&
		kn.Spec.Web.NodePort.HTTPS.Enabled {
		config += buildHTTPSServerBlock(kn)
	}

	return config
}

// containsDot 检查字符串是否包含点号
func containsDot(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}

// buildHTTPServerBlock 构建 HTTP server 块
func buildHTTPServerBlock(kn *kubenovav1.KubeNova) string {
	config := `
# Main HTTP server block
server {
    listen 80;
    server_name _;
`

	// 如果是 NodePort + HTTPS 模式，HTTP 强制跳转到 HTTPS
	if kn.Spec.Web.ExposeType == "nodeport" &&
		kn.Spec.Web.NodePort != nil &&
		kn.Spec.Web.NodePort.HTTPS != nil &&
		kn.Spec.Web.NodePort.HTTPS.Enabled {
		config += `
    # 强制跳转到 HTTPS
    return 301 https://$host$request_uri;
}
`
		return config
	}

	// 正常的 HTTP server 配置
	config += `
    # Root directory
    root /usr/share/nginx/html;
    index index.html;

    # Charset
    charset utf-8;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;
    add_header Permissions-Policy "geolocation=(), microphone=(), camera=()" always;

    # Rate limiting
    limit_req zone=general burst=200 nodelay;
    limit_conn addr 20;

    # Health check endpoint
    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
`

	// 添加所有 location 块
	config += buildLocationBlocks(kn)

	config += `}
`
	return config
}

// buildHTTPSServerBlock 构建 HTTPS server 块
func buildHTTPSServerBlock(kn *kubenovav1.KubeNova) string {
	config := `
# HTTPS server block (NodePort mode)
server {
    listen 443 ssl http2;
    server_name _;

    # SSL configuration
    ssl_certificate /etc/nginx/certs/tls.crt;
    ssl_certificate_key /etc/nginx/certs/tls.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Root directory
    root /usr/share/nginx/html;
    index index.html;

    # Charset
    charset utf-8;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;
    add_header Permissions-Policy "geolocation=(), microphone=(), camera=()" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # Rate limiting
    limit_req zone=general burst=200 nodelay;
    limit_conn addr 20;

    # Health check endpoint
    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
`

	// 添加所有 location 块
	config += buildLocationBlocks(kn)

	config += `}
`
	return config
}

// buildLocationBlocks 构建所有 location 块
func buildLocationBlocks(kn *kubenovav1.KubeNova) string {
	config := `
    # WebSocket proxy for console pod
    location /ws/v1/pod {
        proxy_pass http://console_api;

        # WebSocket specific headers
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;

        # Timeouts for WebSocket
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;

        # Buffering
        proxy_buffering off;
        proxy_request_buffering off;

        # Rate limiting for WebSocket
        limit_req zone=api burst=10 nodelay;
    }

    # WebSocket proxy for portal site messages
    location /ws/v1/site-messages {
        proxy_pass http://portal_api;

        # WebSocket specific headers
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;

        # Timeouts for WebSocket
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;

        # Buffering
        proxy_buffering off;
        proxy_request_buffering off;

        # Rate limiting for WebSocket
        limit_req zone=api burst=10 nodelay;
    }

    # Portal API proxy
    location /portal {
        proxy_pass http://portal_api;

        # Proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;

        # HTTP version
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;

        # Buffering
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
        proxy_busy_buffers_size 8k;

        # Error handling
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503 http_504;
        proxy_next_upstream_tries 2;

        # Rate limiting
        limit_req zone=api burst=20 nodelay;
    }

    # Manager API proxy
    location /manager {
        proxy_pass http://manager_api;

        # Proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;

        # HTTP version
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;

        # Buffering
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
        proxy_busy_buffers_size 8k;

        # Error handling
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503 http_504;
        proxy_next_upstream_tries 2;

        # Rate limiting
        limit_req zone=api burst=20 nodelay;
    }

    # Workload API proxy
    location /workload {
        proxy_pass http://workload_api;

        # Proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;

        # HTTP version
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;

        # Buffering
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
        proxy_busy_buffers_size 8k;

        # Error handling
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503 http_504;
        proxy_next_upstream_tries 2;

        # Rate limiting
        limit_req zone=api burst=20 nodelay;
    }

    # Console API proxy
    location /console {
        proxy_pass http://console_api;

        # Proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;

        # HTTP version
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeouts - longer for console operations
        proxy_connect_timeout 600s;
        proxy_send_timeout 600s;
        proxy_read_timeout 600s;

        # Buffering - disabled for streaming responses
        proxy_buffering off;
        proxy_request_buffering off;

        # Error handling
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503 http_504;
        proxy_next_upstream_tries 2;

        # Rate limiting
        limit_req zone=api burst=20 nodelay;
    }
`

	if kn.IsMinIOProxyEnabled() {
		pathPrefix := kn.GetMinIOProxyPath()

		// 根据 MinIO TLS 配置决定 proxy_pass 协议
		proxyScheme := "http"
		if kn.Spec.Storage.TLS != nil && kn.Spec.Storage.TLS.Enabled {
			proxyScheme = "https"
		}

		config += fmt.Sprintf(`
    # MinIO proxy - 代理到实际 MinIO 地址
    # 协议: %s (根据 MinIO TLS 配置自动选择)
    location %s/ {
        rewrite ^%s/(.*)$ /$1 break;
        proxy_pass %s://minio_backend;
        
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;
        
        proxy_set_header X-Forwarded-Server $host;
        proxy_set_header X-NginX-Proxy true;
        
        # HTTP 版本和连接设置
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Upgrade $http_upgrade;

        # 大文件上传支持
        client_max_body_size 5000m;
        proxy_connect_timeout 600s;
        proxy_send_timeout 600s;
        proxy_read_timeout 600s;

        proxy_buffering off;
        proxy_request_buffering off;
        
        # 错误处理
        proxy_next_upstream error timeout http_502 http_503 http_504;
        proxy_next_upstream_tries 2;
        
        access_log /var/log/nginx/minio_access.log main;
        error_log /var/log/nginx/minio_error.log warn;
`, proxyScheme, pathPrefix, pathPrefix, proxyScheme)

		// 如果启用了 TLS，添加 SSL 验证配置
		if kn.Spec.Storage.TLS != nil && kn.Spec.Storage.TLS.Enabled {
			config += `
        # MinIO TLS 配置
        proxy_ssl_verify off;
        proxy_ssl_server_name on;
        proxy_ssl_protocols TLSv1.2 TLSv1.3;
        proxy_ssl_session_reuse on;
`
		}

		config += `    }
`
	}

	config += `
    # Static assets - cache for 1 year
    location ~* ^/(?!storage/).*\.(jpg|jpeg|png|gif|ico|svg|webp|woff|woff2|ttf|eot|otf)$ {
		expires 1y;
		add_header Cache-Control "public, immutable";
		access_log off;


        # CORS for fonts
        location ~* \.(woff|woff2|ttf|eot|otf)$ {
            add_header Access-Control-Allow-Origin "*";
        }
    }

    # JavaScript and CSS - cache for 1 year with revalidation
    location ~* \.(js|css)$ {
        expires 1y;
        add_header Cache-Control "public, must-revalidate";
        access_log off;
    }

    # HTML files - no cache
    location ~* \.html$ {
        expires -1;
        add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0";
    }

    # Favicon
    location = /favicon.ico {
        log_not_found off;
        access_log off;
        expires 1y;
    }

    # Robots
    location = /robots.txt {
        log_not_found off;
        access_log off;
    }

    # Deny access to hidden files
    location ~ /\. {
        deny all;
        access_log off;
        log_not_found off;
    }

    # Vue Router - SPA fallback
    location / {
        try_files $uri $uri/ /index.html;

        # Cache control for HTML
        add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0";

        # Security headers
        add_header X-Frame-Options "SAMEORIGIN" always;
        add_header X-Content-Type-Options "nosniff" always;
        add_header X-XSS-Protection "1; mode=block" always;
    }

    # Error pages
    error_page 404 /index.html;
    error_page 500 502 503 504 /50x.html;

    location = /50x.html {
        root /usr/share/nginx/html;
        internal;
    }
`

	return config
}
