import './app.css';
import { mount } from 'svelte';
import TopologyStyleOptions from './features/project/TopologyStyleOptions.svelte';

const target = document.getElementById('app');
if (!target) throw new Error('Target was not found');
mount(TopologyStyleOptions, { target });
