import express from 'express';
import path from 'path';
import fs from 'fs';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();
const port = parseInt(process.env.PORT || '3000');
const workerId = process.env.WORKER_PORT || 'unknown';

app.use(express.json());

// In-memory data store
interface Item {
    id: string;
    name: string;
    description: string;
}

let items: Item[] = [
    { id: '1', name: 'Item 1', description: 'Description for Item 1' },
    { id: '2', name: 'Item 2', description: 'Description for Item 2' }
];

// Logging middleware
app.use((req, res, next) => {
    console.log(`[Worker ${workerId}] ${req.method} ${req.url}`);
    next();
});

// Health check
app.get('/health', (req, res) => {
    res.json({ status: 'ok', worker: workerId });
});

// List items
app.get('/items', (req, res) => {
    res.json(items);
});

// Get item
app.get('/items/:id', (req, res) => {
    const item = items.find(i => i.id === req.params.id);
    if (item) {
        res.json(item);
    } else {
        res.status(404).json({ error: 'Item not found' });
    }
});

// Create item
app.post('/items', (req, res) => {
    const newItem: Item = {
        id: (items.length + 1).toString(),
        name: req.body.name,
        description: req.body.description
    };
    items.push(newItem);
    res.status(201).json(newItem);
});

app.get('/bench', (req, res) => {
    res.set('Content-Type', 'text/plain');
    res.send('hello world');
});

app.use(express.static(path.join(__dirname, 'public')));

app.get('/', (req, res) => {
    const templatePath = path.join(__dirname, 'views', 'index.html');
    try {
        let html = fs.readFileSync(templatePath, 'utf8');

        // Replace placeholders
        html = html.replace(/{{\s*PageTitle\s*}}/g, 'API Worker');
        html = html.replace(/{{\s*Path\s*}}/g, process.env.WORKER_PATH || '');
        html = html.replace(/{{\s*Name\s*}}/g, process.env.WORKER_NAME || '');
        html = html.replace(/{{\s*Type\s*}}/g, process.env.WORKER_TYPE || '');
        html = html.replace(/{{\s*Port\s*}}/g, process.env.WORKER_PORT || '');
        html = html.replace(/{{\s*Method\s*}}/g, req.method);
        html = html.replace(/{{\s*URI\s*}}/g, req.url);
        html = html.replace(/{{\s*Time\s*}}/g, new Date().toLocaleString());

        // Always inject dev reload script for now as requested by user context (dev environment)
        if (process.env.WORKER_MODE === 'dev') {
            html = html.replace(/{{\s*DevReload\s*}}/g, '<script src="/dev-reload.js"></script>');
        } else {
            html = html.replace(/{{\s*DevReload\s*}}/g, '');
        }

        res.set('Content-Type', 'text/html');
        res.send(html);
    } catch (err) {
        console.error('Error serving template:', err);
        res.status(500).send('Error serving template');
    }
});

// Start server
app.listen(port, () => {
    console.log(`API Worker listening on port ${port}`);
});
