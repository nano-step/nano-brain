import express from 'express';
import { authMiddleware, corsMiddleware, loggerMiddleware } from './middleware';
import { userController } from './controllers/user';
import { postController } from './controllers/post';
import { healthController } from './controllers/health';

const app = express();
const router = express.Router();

app.use(corsMiddleware);
app.use(loggerMiddleware);

router.get('/users', userController.list);
router.post('/users', userController.create);
router.get('/users/:id', userController.getById);
router.put('/users/:id', userController.update);
router.delete('/users/:id', userController.delete);

router.get('/posts', postController.list);
router.post('/posts', authMiddleware, postController.create);
router.get('/posts/:id', postController.getById);

app.use('/api', router);

app.get('/health', healthController.check);
app.get('/version', healthController.version);

app.use((req, res) => {
  res.status(404).json({ error: 'Not Found' });
});

export default app;
