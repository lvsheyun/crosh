/**
 * Cloudflare Worker for crosh CDN
 * Proxies GitHub Release Assets to provide fast access in mainland China
 * 
 * Routes:
 * - /api/version - Returns latest version from crosh GitHub API
 * - /dist/* - Serves crosh binaries from crosh Release Assets (boomyao/crosh)
 * - /xray/* - Serves Xray-core files from Xray Release Assets (XTLS/Xray-core)
 *             Also serves geoip.dat and geosite.dat from Loyalsoldier/v2ray-rules-dat
 * - /scripts/* - Serves scripts from crosh main branch
 */

const REPO = 'boomyao/crosh';
const XRAY_REPO = 'XTLS/Xray-core';
const GEO_REPO = 'Loyalsoldier/v2ray-rules-dat';
const GITHUB_RAW = 'https://raw.githubusercontent.com';
const GITHUB_API = 'https://api.github.com';

// Cache durations
const CACHE_DURATIONS = {
  version: 3600,     // 1 hour for version API
  binary: 86400,     // 24 hours for binaries
  script: 3600,      // 1 hour for scripts
  data: 86400,       // 24 hours for data files
};

addEventListener('fetch', event => {
  event.respondWith(handleRequest(event.request));
});

async function handleRequest(request) {
  const url = new URL(request.url);
  const path = url.pathname;

  try {
    // Route: /api/version - Get latest version
    if (path === '/api/version') {
      return await handleVersionAPI(request);
    }

    // Route: /dist/* - Serve crosh binaries from GitHub Release Assets
    if (path.startsWith('/dist/')) {
      const filename = path.substring(6); // Remove '/dist/'
      return await proxyReleaseAsset(filename, request, REPO);
    }

    // Route: /xray/* - Serve Xray-core files from GitHub Release Assets
    if (path.startsWith('/xray/')) {
      const filename = path.substring(6); // Remove '/xray/'
      // geoip.dat and geosite.dat come from a different repository
      const isGeoFile = filename === 'geoip.dat' || filename === 'geosite.dat';
      const repo = isGeoFile ? GEO_REPO : XRAY_REPO;
      return await proxyReleaseAsset(filename, request, repo);
    }

    // Route: /scripts/* - Serve scripts from main branch
    if (path.startsWith('/scripts/')) {
      const filename = path.substring(9); // Remove '/scripts/'
      return await proxyGitHubFile('main', `scripts/${filename}`, 'text/plain', CACHE_DURATIONS.script, request);
    }

    // Root path - show usage
    if (path === '/' || path === '') {
      return new Response(getUsageHTML(), {
        headers: {
          'Content-Type': 'text/html;charset=UTF-8',
          'Cache-Control': 'public, max-age=3600',
        },
      });
    }

    return new Response('Not Found', { status: 404 });
  } catch (error) {
    return new Response(`Error: ${error.message}`, { 
      status: 500,
      headers: { 'Content-Type': 'text/plain' }
    });
  }
}

/**
 * Check if cache should be bypassed based on request
 * @param {Request} request - The incoming request
 * @returns {boolean} - True if cache should be bypassed
 */
function shouldBypassCache(request) {
  const url = new URL(request.url);
  
  // Check for nocache query parameter
  if (url.searchParams.has('nocache') || url.searchParams.has('refresh')) {
    return true;
  }
  
  // Check for Cache-Control: no-cache header
  const cacheControl = request.headers.get('Cache-Control');
  if (cacheControl && (cacheControl.includes('no-cache') || cacheControl.includes('no-store'))) {
    return true;
  }
  
  return false;
}

/**
 * Handle version API request
 * Returns the latest version tag from GitHub
 */
async function handleVersionAPI(request) {
  const bypassCache = shouldBypassCache(request);
  const cacheKey = new Request(request.url, request);
  const cache = caches.default;

  // Try to get from cache (unless bypassing)
  if (!bypassCache) {
    let response = await cache.match(cacheKey);
    if (response) {
      // Add header to indicate cache hit
      response = new Response(response.body, response);
      response.headers.set('X-Cache-Status', 'HIT');
      return response;
    }
  }

  // Fetch from GitHub API
  const apiUrl = `${GITHUB_API}/repos/${REPO}/releases/latest`;
  const apiResponse = await fetch(apiUrl, {
    headers: {
      'User-Agent': 'crosh-cdn-worker',
    },
  });

  if (!apiResponse.ok) {
    return new Response('Failed to fetch version', { status: 502 });
  }

  const data = await apiResponse.json();
  const version = data.tag_name;

  // Create response with proper format
  response = new Response(JSON.stringify({ version }), {
    headers: {
      'Content-Type': 'application/json',
      'Cache-Control': `public, max-age=${CACHE_DURATIONS.version}`,
      'Access-Control-Allow-Origin': '*',
      'X-Cache-Status': bypassCache ? 'BYPASS' : 'MISS',
    },
  });

  // Store in cache (even when bypassing, update the cache)
  await cache.put(cacheKey, response.clone());
  return response;
}

/**
 * Proxy a release asset from GitHub Releases
 * Supports both formats:
 * - filename only: uses /releases/latest/download/
 * - version/filename: uses /releases/download/{version}/
 * @param {string} filename - Name of the asset file (can include version like "v1.0.0/file.zip")
 * @param {Request} request - Original request for caching
 * @param {string} repo - GitHub repository in format 'owner/repo'
 */
async function proxyReleaseAsset(filename, request, repo) {
  const bypassCache = shouldBypassCache(request);
  const cacheKey = new Request(request.url, request);
  const cache = caches.default;

  // Try to get from cache (unless bypassing)
  if (!bypassCache) {
    let response = await cache.match(cacheKey);
    if (response) {
      // Add header to indicate cache hit
      response = new Response(response.body, response);
      response.headers.set('X-Cache-Status', 'HIT');
      return response;
    }
  }

  // Parse filename to check if it contains version
  // Format can be: "file.zip" or "v1.0.0/file.zip"
  let downloadUrl;
  let actualFilename = filename;
  
  const parts = filename.split('/');
  if (parts.length === 2) {
    // Format: version/filename
    const version = parts[0];
    actualFilename = parts[1];
    downloadUrl = `https://github.com/${repo}/releases/download/${version}/${actualFilename}`;
  } else {
    // Format: filename only - use latest
    downloadUrl = `https://github.com/${repo}/releases/latest/download/${filename}`;
  }
  
  // Download the asset from GitHub
  const assetResponse = await fetch(downloadUrl, {
    headers: {
      'User-Agent': 'crosh-cdn-worker',
    },
  });

  if (!assetResponse.ok) {
    if (assetResponse.status === 404) {
      return new Response(`Asset not found: ${filename} in ${repo}`, { 
        status: 404,
        headers: { 'Content-Type': 'text/plain' }
      });
    }
    return new Response(`Failed to download asset from ${repo}`, { status: 502 });
  }

  // Create response with appropriate headers
  response = new Response(assetResponse.body, {
    headers: {
      'Content-Type': 'application/octet-stream',
      'Cache-Control': `public, max-age=${CACHE_DURATIONS.binary}`,
      'Access-Control-Allow-Origin': '*',
      'Content-Disposition': `attachment; filename="${actualFilename}"`,
      'X-Cache-Status': bypassCache ? 'BYPASS' : 'MISS',
    },
  });

  // Store in cache (even when bypassing, update the cache)
  await cache.put(cacheKey, response.clone());
  return response;
}

/**
 * Proxy a file from GitHub repository
 * @param {string} branch - Branch name (e.g., main)
 * @param {string} filePath - File path within the repository
 * @param {string} contentType - MIME type for the response
 * @param {number} cacheDuration - Cache duration in seconds
 * @param {Request} request - Original request for cache bypass check
 */
async function proxyGitHubFile(branch, filePath, contentType, cacheDuration, request = null) {
  const bypassCache = request ? shouldBypassCache(request) : false;
  
  // Construct GitHub raw URL
  const githubUrl = `${GITHUB_RAW}/${REPO}/${branch}/${filePath}`;
  
  // Create cache key
  const cacheKey = new Request(githubUrl);
  const cache = caches.default;

  // Try to get from cache (unless bypassing)
  if (!bypassCache) {
    let response = await cache.match(cacheKey);
    if (response) {
      // Add CORS headers to cached response
      response = new Response(response.body, response);
      response.headers.set('Access-Control-Allow-Origin', '*');
      response.headers.set('X-Cache-Status', 'HIT');
      return response;
    }
  }

  // Fetch from GitHub
  const githubResponse = await fetch(githubUrl, {
    headers: {
      'User-Agent': 'crosh-cdn-worker',
    },
  });

  if (!githubResponse.ok) {
    return new Response(`File not found: ${filePath}`, { status: 404 });
  }

  // Create response with appropriate headers
  response = new Response(githubResponse.body, {
    headers: {
      'Content-Type': contentType,
      'Cache-Control': `public, max-age=${cacheDuration}`,
      'Access-Control-Allow-Origin': '*',
      'Content-Disposition': `inline; filename="${filePath.split('/').pop()}"`,
      'X-Cache-Status': bypassCache ? 'BYPASS' : 'MISS',
    },
  });

  // Store in cache (even when bypassing, update the cache)
  await cache.put(cacheKey, response.clone());
  return response;
}

/**
 * Generate usage HTML for root path
 */
function getUsageHTML() {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>crosh CDN - Cloudflare Worker</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      max-width: 800px;
      margin: 50px auto;
      padding: 0 20px;
      line-height: 1.6;
      color: #333;
    }
    h1 { color: #0066cc; }
    h2 { color: #444; margin-top: 30px; }
    code {
      background: #f4f4f4;
      padding: 2px 6px;
      border-radius: 3px;
      font-family: "Courier New", monospace;
    }
    pre {
      background: #f4f4f4;
      padding: 15px;
      border-radius: 5px;
      overflow-x: auto;
    }
    .endpoint {
      margin: 10px 0;
      padding: 10px;
      background: #f9f9f9;
      border-left: 3px solid #0066cc;
    }
    a { color: #0066cc; }
  </style>
</head>
<body>
  <h1>crosh CDN</h1>
  <p>Cloudflare Worker CDN for crosh - Network acceleration tool for Chinese developers</p>

  <h2>Installation</h2>
  <pre>curl -fsSL https://crosh.boomyao.com/scripts/install.sh | bash</pre>

  <h2>Available Endpoints</h2>

  <div class="endpoint">
    <strong>GET /api/version</strong><br>
    Returns the latest version from GitHub releases
  </div>

  <div class="endpoint">
    <strong>GET /dist/{binary}</strong><br>
    Download crosh binaries from latest GitHub Release (e.g., <code>crosh-linux-amd64</code>, <code>crosh-darwin-arm64</code>)
  </div>

  <div class="endpoint">
    <strong>GET /xray/{file}</strong><br>
    Download Xray-core binaries and data files from XTLS/Xray-core latest release (e.g., <code>Xray-linux-64.zip</code>, <code>geoip.dat</code>)
  </div>

  <div class="endpoint">
    <strong>GET /scripts/{script}</strong><br>
    Download installation scripts (e.g., <code>install.sh</code>)
  </div>

  <h2>Cache Bypass</h2>
  <p>To bypass cache and force fetch the latest version, add <code>?nocache=1</code> or <code>?refresh=1</code> to any URL:</p>
  <pre># Force refresh version info
curl https://crosh.boomyao.com/api/version?nocache=1

# Force download latest binary
curl https://crosh.boomyao.com/dist/crosh-linux-amd64?nocache=1</pre>

  <h2>Examples</h2>
  <pre># Get latest version
curl https://crosh.boomyao.com/api/version

# Download install script
curl https://crosh.boomyao.com/scripts/install.sh

# Download binary
curl -O https://crosh.boomyao.com/dist/crosh-linux-amd64</pre>

  <h2>GitHub Repository</h2>
  <p><a href="https://github.com/${REPO}">https://github.com/${REPO}</a></p>
</body>
</html>`;
}

