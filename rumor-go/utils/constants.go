package utils

const MantleKeyTag = "mantle"
const MantleModelTag = "model"
const MantleQueryTag = "query"
const GraphQLAllowedCharactersRegex = "^[_A-Za-z][_0-9A-Za-z]*$"

const DepsResolverKey = "graph::depsresolver"
const QuerierKey = "graph::querier"
const ImmediateResolveFlagKey = "graph::resolve_immediately"
const DependenciesKey = "graph::dependencies"
const ProxyResolverContextKey = "graph::proxy_resolver_context"

type DependenciesKeyType map[string]bool
