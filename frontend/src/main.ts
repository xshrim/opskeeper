import './app.css';
import './layouts/layout.css';
import './components/components.css';
import './features/auth/auth.css';
import './features/diagnosis/diagnosis.css';
import './features/resource/resource.css';
import './features/access/access.css';
import './features/discovery/discovery.css';
import './features/project/project.css';
import './features/profile/profile.css';
import './features/skill/skill.css';
import { mount } from 'svelte';
import App from './App.svelte';

const target = document.getElementById('app');

if (!target) {
  throw new Error('Application target was not found');
}

mount(App, { target });
