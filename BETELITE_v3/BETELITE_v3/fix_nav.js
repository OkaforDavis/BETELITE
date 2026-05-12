const fs = require('fs');
let content = fs.readFileSync('mobile/index.html', 'utf8');

// 1. Update #main
content = content.replace(
  /#main\s*\{\s*flex:\s*1;\s*overflow-y:\s*auto;\s*-webkit-overflow-scrolling:\s*touch;\s*\/\* leave space so content isn't behind nav \*\/\s*padding-bottom:\s*0;\s*\}/,
  `#main {
  flex: 1; overflow-y: auto; -webkit-overflow-scrolling: touch;
  /* leave space so content isn't behind nav */
  padding-bottom: calc(var(--nav-h) + max(env(safe-area-inset-bottom, 0px), 6px) + 20px);
}`
);

// 2. Update #nav CSS
content = content.replace(
  /#nav\s*\{\s*flex-shrink:\s*0;\s*display:\s*flex;\s*background:\s*rgba\(13,\s*15,\s*28,\s*0\.7\);\s*backdrop-filter:\s*blur\(15px\);\s*-webkit-backdrop-filter:\s*blur\(15px\);\s*border-top:\s*1px\s*solid\s*var\(--border\);\s*z-index:\s*20;\s*padding-bottom:\s*max\(env\(safe-area-inset-bottom,\s*0px\),\s*6px\);\s*min-height:\s*calc\(var\(--nav-h\)\s*\+\s*max\(env\(safe-area-inset-bottom,\s*0px\),\s*6px\)\);\s*position:\s*relative;\s*width:\s*100%;\s*max-width:\s*430px;\s*margin:\s*0\s*auto;\s*\}/,
  `#nav {
  display: flex;
  background: rgba(13, 15, 28, 0.7);
  backdrop-filter: blur(15px);
  -webkit-backdrop-filter: blur(15px);
  border-top: 1px solid var(--border);
  z-index: 20;
  padding-bottom: max(env(safe-area-inset-bottom, 0px), 6px);
  min-height: calc(var(--nav-h) + max(env(safe-area-inset-bottom, 0px), 6px));
  position: fixed; bottom: 0; left: 50%; transform: translateX(-50%);
  width: 100%; max-width: 430px;
}`
);

fs.writeFileSync('mobile/index.html', content);
console.log('Nav bar fixed.');
