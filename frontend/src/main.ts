import './app.css';
import './features/diagnosis/diagnosis.css';
import { mount } from 'svelte';
import App from './App.svelte';

const target = document.getElementById('app');

if (!target) {
  throw new Error('Application target was not found');
}

mount(App, { target });
