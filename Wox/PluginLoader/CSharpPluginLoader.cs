﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using Wox.Helper;
using Wox.Plugins;

namespace Wox.PluginLoader {

	public class CSharpPluginLoader : BasePluginLoader {

		public override List<PluginPair> LoadPlugin() {
			var plugins = new List<PluginPair>();

			List<PluginMetadata> metadatas = pluginMetadatas.Where(o => o.Language.ToUpper() == AllowedLanguage.CSharp.ToUpper()).ToList();
			foreach (PluginMetadata metadata in metadatas) {
				try {
					Assembly asm = Assembly.Load(AssemblyName.GetAssemblyName(metadata.ExecuteFilePath));
					List<Type> types = asm.GetTypes().Where(o => o.IsClass && !o.IsAbstract && (o.BaseType == typeof(BaseSystemPlugin) || o.GetInterfaces().Contains(typeof(IPlugin)))).ToList();
					if (types.Count == 0) {
						Log.Warn(string.Format("Couldn't load plugin {0}: didn't find the class who implement IPlugin", metadata.Name));
						continue;
					}

					foreach (Type type in types) {
						PluginPair pair = new PluginPair() {
							Plugin = Activator.CreateInstance(type) as IPlugin,
							Metadata = metadata
						};

						var sys = pair.Plugin as BaseSystemPlugin;
						if (sys != null) {
							sys.PluginDirectory = metadata.PluginDirecotry;
						}

						plugins.Add(pair);
					}
				}
				catch (Exception e) {
					Log.Error(string.Format("Couldn't load plugin {0}: {1}", metadata.Name, e.Message));
#if (DEBUG)
					{
						throw;
					}
#endif
				}
			}

			return plugins;
		}
	}
}