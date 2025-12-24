#!/usr/bin/env python3
"""
ManticoreSearch 查询工具服务器
解决 CORS 问题，提供代理功能
"""

import http.server
import urllib.request
import urllib.parse
import json
import os

PORT = 8080
MANTICORE_URL = os.environ.get('MANTICORE_URL', 'http://localhost:30399/sql')

class ProxyHandler(http.server.SimpleHTTPRequestHandler):
    def do_POST(self):
        # 代理 /api/sql 请求到 ManticoreSearch
        if self.path == '/api/sql':
            try:
                content_length = int(self.headers.get('Content-Length', 0))
                body = self.rfile.read(content_length)
                
                # 转发请求到 ManticoreSearch
                req = urllib.request.Request(
                    MANTICORE_URL,
                    data=body,
                    headers={'Content-Type': 'application/x-www-form-urlencoded'}
                )
                
                with urllib.request.urlopen(req, timeout=10) as resp:
                    result = resp.read()
                
                # 返回响应（带 CORS 头）
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.end_headers()
                self.wfile.write(result)
                
            except Exception as e:
                self.send_response(500)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.end_headers()
                self.wfile.write(json.dumps({'error': str(e)}).encode())
        else:
            self.send_response(404)
            self.end_headers()
    
    def do_OPTIONS(self):
        # 处理 CORS 预检请求
        self.send_response(200)
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type')
        self.end_headers()
    
    def log_message(self, format, *args):
        # 简化日志
        if '/api/' in args[0]:
            print(f"[PROXY] {args[0]}")

if __name__ == '__main__':
    os.chdir(os.path.dirname(os.path.abspath(__file__)))
    
    print(f"""
╔══════════════════════════════════════════════════════════╗
║     ManticoreSearch 查询工具服务器                        ║
╚══════════════════════════════════════════════════════════╝

🌐 访问地址: http://localhost:{PORT}/manticore-query.html
🔗 代理端点: http://localhost:{PORT}/api/sql
📡 ManticoreSearch: {MANTICORE_URL}

按 Ctrl+C 停止服务器
""")
    
    server = http.server.HTTPServer(('', PORT), ProxyHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n服务器已停止")

