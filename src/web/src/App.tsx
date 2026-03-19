import { Navigate, Route, Routes } from 'react-router-dom';
import Layout from './components/Layout';
import Dashboard from './views/Dashboard';
import GraphExplorer from './views/GraphExplorer';
import CodeGraph from './views/CodeGraph';
import Search from './views/Search';

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/graph" element={<GraphExplorer />} />
        <Route path="/code" element={<CodeGraph />} />
        <Route path="/search" element={<Search />} />
      </Routes>
    </Layout>
  );
}
