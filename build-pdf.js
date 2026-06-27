const { mdToPdf } = require('md-to-pdf');

(async () => {
    try {
        console.log("Starting PDF generation...");
        const pdf = await mdToPdf({ path: 'system_architecture_flowchart.md' }, {
            dest: 'system_architecture_flowchart.pdf',
            stylesheet: ['dark.css'],
            body_class: ['markdown-body'],
            pdf_options: {
                format: 'A4',
                margin: '20mm',
                printBackground: true,
                args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu', '--single-process']
            },
            mermaid_options: {
                theme: 'dark'
            }
        }).catch(err => {
            console.error("Error during generation:", err);
            process.exit(1);
        });
        console.log("PDF successfully generated!");
    } catch (err) {
        console.error("Failed:", err);
    }
})();
