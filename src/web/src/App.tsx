import { Navigate, Route, Routes } from 'react-router-dom';
import Layout from './components/Layout';
import ErrorBoundary from './components/ErrorBoundary';
import Dashboard from './views/Dashboard';
import GraphExplorer from './views/GraphExplorer';
import CodeGraph from './views/CodeGraph';
import SymbolGraph from './views/SymbolGraph';
import FlowsView from './views/FlowsView';
import ConnectionsView from './views/ConnectionsView';
import InfrastructureView from './views/InfrastructureView';
import Search from './views/Search';

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<ErrorBoundary fallbackTitle="Dashboard error"><Dashboard /></ErrorBoundary>} />
        <Route path="/graph" element={<ErrorBoundary fallbackTitle="Knowledge Graph error"><GraphExplorer /></ErrorBoundary>} />
        <Route path="/code" element={<ErrorBoundary fallbackTitle="Code Graph error"><CodeGraph /></ErrorBoundary>} />
        <Route path="/symbols" element={<ErrorBoundary fallbackTitle="Symbol Graph error"><SymbolGraph /></ErrorBoundary>} />
        <Route path="/flows" element={<ErrorBoundary fallbackTitle="Flows error"><FlowsView /></ErrorBoundary>} />
        <Route path="/connections" element={<ErrorBoundary fallbackTitle="Connections error"><ConnectionsView /></ErrorBoundary>} />
        <Route path="/infrastructure" element={<ErrorBoundary fallbackTitle="Infrastructure error"><InfrastructureView /></ErrorBoundary>} />
        <Route path="/search" element={<ErrorBoundary fallbackTitle="Search error"><Search /></ErrorBoundary>} />
      </Routes>
    </Layout>
  );
}
