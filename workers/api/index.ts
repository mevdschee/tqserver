import express from 'express';

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

// Start server
app.listen(port, () => {
    console.log(`API Worker listening on port ${port}`);
});
