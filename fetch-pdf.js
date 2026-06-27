const fs = require('fs');

(async () => {
    try {
        console.log("Sending to md-to-pdf API...");
        const markdown = fs.readFileSync('system_architecture_flowchart.md', 'utf8');
        const css = fs.readFileSync('dark.css', 'utf8');
        
        const response = await fetch('https://md-to-pdf.fly.dev/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({
                markdown: markdown,
                css: css,
                engine: 'puppeteer'
            })
        });

        if (!response.ok) {
            console.error("API error:", response.status, await response.text());
            process.exit(1);
        }

        const arrayBuffer = await response.arrayBuffer();
        fs.writeFileSync('BETELITE_System_Process_Flowchart.pdf', Buffer.from(arrayBuffer));
        console.log("Successfully saved BETELITE_System_Process_Flowchart.pdf!");
    } catch (err) {
        console.error("Fetch failed:", err);
    }
})();
