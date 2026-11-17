// Photo backdrops shipped with the app: CC0 pictures from Wikimedia Commons,
// re-encoded at 2400 px (see assets/backdrops/CREDITS.md). colors is the
// photo's average colour, shown until the photo is decoded.
// Generated from the curation list; edit the list, not this file by hand.

import type { BackdropPreset } from './backdrops';
import mistyLake from '../assets/backdrops/misty-lake.jpg';
import mistyLakeThumb from '../assets/backdrops/misty-lake-thumb.jpg';
import cloudSea from '../assets/backdrops/cloud-sea.jpg';
import cloudSeaThumb from '../assets/backdrops/cloud-sea-thumb.jpg';
import aurora from '../assets/backdrops/aurora.jpg';
import auroraThumb from '../assets/backdrops/aurora-thumb.jpg';
import dunes from '../assets/backdrops/dunes.jpg';
import dunesThumb from '../assets/backdrops/dunes-thumb.jpg';
import dawn from '../assets/backdrops/dawn.jpg';
import dawnThumb from '../assets/backdrops/dawn-thumb.jpg';
import islandFog from '../assets/backdrops/island-fog.jpg';
import islandFogThumb from '../assets/backdrops/island-fog-thumb.jpg';
import foggyHills from '../assets/backdrops/foggy-hills.jpg';
import foggyHillsThumb from '../assets/backdrops/foggy-hills-thumb.jpg';
import redSky from '../assets/backdrops/red-sky.jpg';
import redSkyThumb from '../assets/backdrops/red-sky-thumb.jpg';
import crete from '../assets/backdrops/crete.jpg';
import creteThumb from '../assets/backdrops/crete-thumb.jpg';
import blueSky from '../assets/backdrops/blue-sky.jpg';
import blueSkyThumb from '../assets/backdrops/blue-sky-thumb.jpg';
import saltLake from '../assets/backdrops/salt-lake.jpg';
import saltLakeThumb from '../assets/backdrops/salt-lake-thumb.jpg';
import mountainDawn from '../assets/backdrops/mountain-dawn.jpg';
import mountainDawnThumb from '../assets/backdrops/mountain-dawn-thumb.jpg';

export const PHOTO_BACKDROPS: BackdropPreset[] = [
  { id: 'misty-lake', name: 'Misty Lake', colors: ['#787e86'], angle: 0, image: mistyLake, thumb: mistyLakeThumb },
  { id: 'cloud-sea', name: 'Sea of Clouds', colors: ['#aaa69f'], angle: 0, image: cloudSea, thumb: cloudSeaThumb },
  { id: 'aurora', name: 'Aurora', colors: ['#282c29'], angle: 0, image: aurora, thumb: auroraThumb },
  { id: 'dunes', name: 'Dunes', colors: ['#bf8d6c'], angle: 0, image: dunes, thumb: dunesThumb },
  { id: 'dawn', name: 'Dawn', colors: ['#908074'], angle: 0, image: dawn, thumb: dawnThumb },
  { id: 'island-fog', name: 'Island Fog', colors: ['#86adcf'], angle: 0, image: islandFog, thumb: islandFogThumb },
  { id: 'foggy-hills', name: 'Foggy Hills', colors: ['#9a988d'], angle: 0, image: foggyHills, thumb: foggyHillsThumb },
  { id: 'red-sky', name: 'Red Sky', colors: ['#a99090'], angle: 0, image: redSky, thumb: redSkyThumb },
  { id: 'crete', name: 'Crete', colors: ['#727c6b'], angle: 0, image: crete, thumb: creteThumb },
  { id: 'blue-sky', name: 'Blue Sky', colors: ['#335db1'], angle: 0, image: blueSky, thumb: blueSkyThumb },
  { id: 'salt-lake', name: 'Salt Lake', colors: ['#697d8c'], angle: 0, image: saltLake, thumb: saltLakeThumb },
  { id: 'mountain-dawn', name: 'Mountain Dawn', colors: ['#7b696b'], angle: 0, image: mountainDawn, thumb: mountainDawnThumb },
];
