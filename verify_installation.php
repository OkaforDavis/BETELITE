<?php
/**
 * BETELITE Installation Verification Script
 * ==========================================
 * Tests core components without requiring database
 */

echo "═══════════════════════════════════════════════════════════\n";
echo "BETELITE Installation Verification\n";
echo "═══════════════════════════════════════════════════════════\n\n";

$tests = [];
$passed = 0;
$failed = 0;

// Test 1: Core files exist
echo "[1] Checking core files...\n";
$coreFiles = [
    '/core/bootstrap.php',
    '/core/security.php',
    '/core/logger.php',
    '/core/validator.php',
    '/core/database.php',
    '/core/router.php',
    '/core/errors.php',
];

foreach ($coreFiles as $file) {
    $path = __DIR__ . $file;
    if (file_exists($path)) {
        echo "  ✓ $file\n";
        $passed++;
    } else {
        echo "  ✗ $file (MISSING)\n";
        $failed++;
    }
}

// Test 2: Service files exist
echo "\n[2] Checking service layer...\n";
$serviceFiles = [
    '/services/AuthService.php',
    '/services/WalletService.php',
    '/services/TournamentService.php',
    '/services/PaymentService.php',
];

foreach ($serviceFiles as $file) {
    $path = __DIR__ . $file;
    if (file_exists($path)) {
        echo "  ✓ $file\n";
        $passed++;
    } else {
        echo "  ✗ $file (MISSING)\n";
        $failed++;
    }
}

// Test 3: Documentation files exist
echo "\n[3] Checking documentation...\n";
$docFiles = [
    '/README.md',
    '/ARCHITECTURE.md',
    '/API.md',
    '/DEPLOYMENT.md',
    '/SECURITY.md',
    '/TESTING.md',
];

foreach ($docFiles as $file) {
    $path = __DIR__ . $file;
    if (file_exists($path)) {
        echo "  ✓ $file\n";
        $passed++;
    } else {
        echo "  ✗ $file (MISSING)\n";
        $failed++;
    }
}

// Test 4: Configuration
echo "\n[4] Checking configuration...\n";
if (file_exists(__DIR__ . '/config.php')) {
    echo "  ✓ config.php\n";
    $passed++;
} else {
    echo "  ✗ config.php (MISSING)\n";
    $failed++;
}

if (file_exists(__DIR__ . '/.env')) {
    echo "  ✓ .env file exists\n";
    $passed++;
} else {
    echo "  ✗ .env file (missing - may be required for production)\n";
}

// Test 5: Database schema
echo "\n[5] Checking database schema...\n";
if (file_exists(__DIR__ . '/database/schema.sql')) {
    $schema = file_get_contents(__DIR__ . '/database/schema.sql');
    $tableCount = substr_count($schema, 'CREATE TABLE');
    echo "  ✓ schema.sql ($tableCount tables)\n";
    $passed++;
} else {
    echo "  ✗ schema.sql (MISSING)\n";
    $failed++;
}

// Test 6: Version information
echo "\n[6] Checking version...\n";
if (file_exists(__DIR__ . '/version.php')) {
    require_once __DIR__ . '/version.php';
    if (isset($VERSION)) {
        echo "  ✓ Version: {$VERSION['version']}\n";
        echo "    Name: {$VERSION['name']}\n";
        echo "    Environment: {$VERSION['env']}\n";
        $passed++;
    }
} else {
    echo "  ✗ version.php (MISSING)\n";
    $failed++;
}

// Test 7: PHP version
echo "\n[7] Checking PHP environment...\n";
echo "  ✓ PHP Version: " . phpversion() . "\n";
echo "  ✓ SAPI: " . php_sapi_name() . "\n";

// Check required extensions
$extensions = ['pdo', 'pdo_mysql', 'json', 'openssl'];
foreach ($extensions as $ext) {
    if (extension_loaded($ext)) {
        echo "  ✓ Extension: $ext\n";
        $passed++;
    } else {
        echo "  ✗ Extension: $ext (NOT LOADED)\n";
        $failed++;
    }
}

// Test 8: Directory structure
echo "\n[8] Checking directory structure...\n";
$dirs = ['/logs', '/database', '/tests', '/api', '/services', '/core'];
foreach ($dirs as $dir) {
    $path = __DIR__ . $dir;
    if (is_dir($path)) {
        $fileCount = count(glob($path . '/*', GLOB_BRACE));
        echo "  ✓ $dir ($fileCount files)\n";
        $passed++;
    } else {
        echo "  ✗ $dir (NOT FOUND)\n";
        $failed++;
    }
}

// Summary
echo "\n═══════════════════════════════════════════════════════════\n";
echo "SUMMARY\n";
echo "═══════════════════════════════════════════════════════════\n";
echo "✓ Passed: $passed\n";
echo "✗ Failed: $failed\n";

if ($failed === 0) {
    echo "\n✅ Installation verified successfully!\n";
    echo "\nNEXT STEPS:\n";
    echo "1. Update .env with your database credentials\n";
    echo "2. Run: mysql < database/schema.sql\n";
    echo "3. Run: php tests/TestFramework.php\n";
    echo "4. Configure your web server (Nginx/Apache)\n";
    echo "5. Deploy to production\n";
} else {
    echo "\n❌ Installation incomplete - $failed checks failed\n";
}

echo "\n═══════════════════════════════════════════════════════════\n";
?>
