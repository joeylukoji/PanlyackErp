[graphify] MultiDiGraph edge-collapse diagnostic
input: <in-memory>
input_stage: provided JSON (normal graph.json is post-build)
effective_directed: <direct-call>
nodes: 298
unverified_code_nodes: 0
raw_edges: 863
valid_candidate_edges: 697
missing_endpoint_edges: 0
dangling_endpoint_edges: 166
external_reference_edges: 0
self_loop_edges: 0
exact_duplicate_edges: 4
directed_unique_endpoint_pairs: 679
directed_same_endpoint_collapsed_edges: 18
undirected_unique_endpoint_pairs: 678
undirected_same_endpoint_collapsed_edges: 19
same_endpoint_group_count: 18
relation_variant_groups: 8
source_file_variant_groups: 0
source_location_variant_groups: 2
context_variant_groups: 4
post_build_graph_type: DiGraph
post_build_edges: 825
producer_suppression_sites: 12
producer_suppression_examples:
  - L1646 seen_ids arity=unknown
  - L2179 seen_ids arity=unknown
  - L2181 seen_doc_refs arity=unknown
  - L2551 seen_ids arity=unknown
  - L2698 seen_ids arity=unknown
  - L3424 seen_keys arity=unknown
  - L3593 seen_keys arity=unknown
  - L5669 seen_ids arity=unknown
examples:
  - ai_service_app_extractors_validate_file -> ai_service_app_schemas_extractionrequest edges=2 relations=['references', 'uses'] locations=['L36'] contexts=['', 'parameter_type']
  - ai_service_app_main_get_extractor -> ai_service_app_extractors_extractor edges=2 relations=['references', 'uses'] locations=['L14'] contexts=['', 'return_type']
  - ai_service_app_main_ready -> ai_service_app_extractors_extractor edges=2 relations=['references', 'uses'] locations=['L35'] contexts=['', 'generic_arg']
  - ai_service_app_main_extract -> ai_service_app_schemas_extractionrequest edges=2 relations=['references', 'uses'] locations=['L52'] contexts=['', 'parameter_type']
  - ai_service_app_main_extract -> ai_service_app_extractors_extractor edges=2 relations=['references', 'uses'] locations=['L52'] contexts=['', 'generic_arg']
note: normal graph.json is post-build; raw producer loss must be measured earlier.