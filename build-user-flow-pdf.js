const { mdToPdf } = require('md-to-pdf');

(async () => {
    try {
        console.log("Starting PDF generation for user flow...");
        const pdf = await mdToPdf({ path: 'user_flow.md' }, {
            dest: 'user_flow.pdf',
            stylesheet: ['dark.css'],
            body_class: ['markdown-body'],
            pdf_options: {
                format: 'A4',
                margin: '20mm',
                printBackground: true,
                args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu', '--single-process']
            },
            mermaid_options: {
                theme: 'dark',
                securityLevel: 'loose'
            }
        }).catch(err => {
            console.error("Error during generation:", err);
            process.exit(1);
        });
        console.log("PDF successfully generated at user_flow.pdf!");
    } catch (err) {
        console.error("Failed:", err);
    }
})();
