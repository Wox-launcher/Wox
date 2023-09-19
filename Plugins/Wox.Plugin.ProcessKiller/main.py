from wox import WoxPlugin, PluginInitContext, Query, PublicAPI


class ProcessKiller(WoxPlugin):
    api: PublicAPI

    def init(self, init_context: PluginInitContext):
        self.api = init_context.api
        self.api.log("init process killer")

    def query(self, query: Query):
        self.api.change_query(query.raw_query)
