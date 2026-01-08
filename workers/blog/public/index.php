<?php

/*
 * Blog Worker - Webserver with PHP via php-fpm
 * This script demonstrates PHP execution through TQServer's php-fpm integration and dynamic pool manager
 */

// Get request information
$requestUri = $_SERVER['REQUEST_URI'] ?? '/';
$requestMethod = $_SERVER['REQUEST_METHOD'] ?? 'GET';
$serverSoftware = $_SERVER['SERVER_SOFTWARE'] ?? 'Unknown';

// Response headers
header('Content-Type: text/html; charset=utf-8');
http_response_code(200);

?>
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TQServer PHP Worker - Blog</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: #333;
        }

        .container {
            background: white;
            border-radius: 10px;
            padding: 40px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
        }

        h1 {
            color: #667eea;
            margin-bottom: 10px;
        }

        .badge {
            display: inline-block;
            background: #48bb78;
            color: white;
            padding: 5px 12px;
            border-radius: 20px;
            font-size: 14px;
            font-weight: bold;
            margin-bottom: 20px;
        }

        .info {
            background: #f7fafc;
            border-left: 4px solid #667eea;
            padding: 15px;
            margin: 20px 0;
            border-radius: 5px;
        }

        .info strong {
            color: #667eea;
        }

        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin: 20px 0;
        }

        .stat {
            background: #edf2f7;
            padding: 15px;
            border-radius: 8px;
            text-align: center;
        }

        .stat-label {
            font-size: 12px;
            color: #718096;
            text-transform: uppercase;
            font-weight: 600;
        }

        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #2d3748;
            margin-top: 5px;
        }

        code {
            background: #2d3748;
            color: #48bb78;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
        }
    </style>
</head>

<body>
    <div class="container">
        <h1>🚀 TQServer PHP Worker</h1>
        <span class="badge">✓ Dynamic Pool Manager</span>

        <div class="info">
            <p><strong>Worker:</strong> blog</p>
            <p><strong>Type:</strong> PHP via FastCGI</p>
            <p><strong>Path:</strong> <?php echo htmlspecialchars($requestUri); ?></p>
            <p><strong>Method:</strong> <?php echo htmlspecialchars($requestMethod); ?></p>
        </div>

        <h2>PHP Runtime Info</h2>
        <div class="stats">
            <div class="stat">
                <div class="stat-label">PHP Version</div>
                <div class="stat-value"><?php echo PHP_VERSION; ?></div>
            </div>
            <div class="stat">
                <div class="stat-label">Memory Limit</div>
                <div class="stat-value"><?php echo ini_get('memory_limit'); ?></div>
            </div>
            <div class="stat">
                <div class="stat-label">Max Execution Time</div>
                <div class="stat-value"><?php echo ini_get('max_execution_time'); ?>s</div>
            </div>
            <div class="stat">
                <div class="stat-label">Upload Max Size</div>
                <div class="stat-value"><?php echo ini_get('upload_max_filesize'); ?></div>
            </div>
        </div>

        <h2>TQServer Features</h2>
        <ul>
            <li>✅ <strong>Proven Runtime:</strong> Runs via industry standard php-fpm adapter.</li>
            <li>✅ <strong>Dynamic Worker Pool:</strong> Automatic scaling based on load</li>
            <li>✅ <strong>Graceful Restart:</strong> Configuration changes without downtime</li>
            <li>✅ <strong>Hot Reload:</strong> Reload web pages on file changes</li>
            <li>✅ <strong>Health Monitoring:</strong> Automatic worker restart on failures</li>
        </ul>

        <h2>Current Request Environment</h2>
        <div class="info">
            <?php foreach (['REQUEST_METHOD', 'REQUEST_URI', 'SERVER_PROTOCOL', 'HTTP_HOST', 'HTTP_USER_AGENT'] as $key): ?>
                <?php if (isset($_SERVER[$key])): ?>
                    <p><strong><?php echo $key; ?>:</strong> <?php echo htmlspecialchars($_SERVER[$key]); ?></p>
                <?php endif; ?>
            <?php endforeach; ?>
        </div>

        <p style="text-align: center; color: #718096; margin-top: 30px;">
            Powered by <strong>TQServer</strong> - Webserver with PHP support via php-fpm
        </p>
    </div>
    <?php if (getenv('WORKER_SERVER_MODE') === 'dev'): ?>
        <script src="/dev-reload.js"></script>
    <?php endif; ?>
</body>

</html>