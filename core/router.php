<?php
/**
 * BETELITE ROUTER
 * ===============
 * Simple, lightweight REST API router
 * No heavy framework dependencies
 */

class Router {
    
    private $routes = [];
    private $middleware = [];
    private $prefix = '';
    
    /**
     * Register GET route
     */
    public function get($path, $handler) {
        return $this->register('GET', $path, $handler);
    }
    
    /**
     * Register POST route
     */
    public function post($path, $handler) {
        return $this->register('POST', $path, $handler);
    }
    
    /**
     * Register PUT route
     */
    public function put($path, $handler) {
        return $this->register('PUT', $path, $handler);
    }
    
    /**
     * Register DELETE route
     */
    public function delete($path, $handler) {
        return $this->register('DELETE', $path, $handler);
    }
    
    /**
     * Register PATCH route
     */
    public function patch($path, $handler) {
        return $this->register('PATCH', $path, $handler);
    }
    
    /**
     * Register route for any method
     */
    public function any($path, $handler) {
        foreach (['GET', 'POST', 'PUT', 'DELETE', 'PATCH'] as $method) {
            $this->register($method, $path, $handler);
        }
        return $this;
    }
    
    /**
     * Register a route
     */
    private function register($method, $path, $handler) {
        $fullPath = $this->prefix . $path;
        $this->routes[] = [
            'method' => strtoupper($method),
            'path' => $fullPath,
            'pattern' => $this->pathToPattern($fullPath),
            'handler' => $handler,
        ];
        return $this;
    }
    
    /**
     * Set route prefix
     */
    public function prefix($prefix, $callback) {
        $oldPrefix = $this->prefix;
        $this->prefix = $oldPrefix . $prefix;
        $callback($this);
        $this->prefix = $oldPrefix;
        return $this;
    }
    
    /**
     * Add global middleware
     */
    public function use($middleware) {
        $this->middleware[] = $middleware;
        return $this;
    }
    
    /**
     * Convert path with parameters to regex pattern
     */
    private function pathToPattern($path) {
        // Convert /api/users/:id to regex pattern
        $pattern = str_replace('/', '\/', $path);
        $pattern = preg_replace('/:([a-zA-Z_][a-zA-Z0-9_]*)/', '(?P<$1>[a-zA-Z0-9_-]+)', $pattern);
        return '/^' . $pattern . '$/';
    }
    
    /**
     * Dispatch request to appropriate handler
     */
    public function dispatch() {
        $method = $_SERVER['REQUEST_METHOD'];
        $path = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
        
        // Remove base path if needed
        $basePath = dirname($_SERVER['SCRIPT_NAME']);
        if ($basePath !== '/' && strpos($path, $basePath) === 0) {
            $path = substr($path, strlen($basePath));
        }
        
        // Find matching route
        foreach ($this->routes as $route) {
            if ($route['method'] === $method && preg_match($route['pattern'], $path, $matches)) {
                // Extract parameters
                $params = [];
                foreach ($matches as $key => $value) {
                    if (!is_numeric($key)) {
                        $params[$key] = $value;
                    }
                }
                
                // Call handler
                return $this->callHandler($route['handler'], $params);
            }
        }
        
        // No route found
        http_response_code(404);
        return json_encode(['error' => 'Route not found']);
    }
    
    /**
     * Call route handler
     */
    private function callHandler($handler, $params = []) {
        if (is_callable($handler)) {
            return call_user_func_array($handler, [$params]);
        } elseif (is_string($handler) && strpos($handler, '@') !== false) {
            // Controller@method format
            list($controller, $method) = explode('@', $handler);
            $instance = new $controller();
            return call_user_func_array([$instance, $method], [$params]);
        }
        
        throw new Exception("Invalid handler format");
    }
}
