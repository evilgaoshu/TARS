#!/usr/bin/env ruby
# frozen_string_literal: true

require 'set'
require 'yaml'

ROOT = File.expand_path('..', __dir__)
OPENAPI_PATH = File.join(ROOT, 'api', 'openapi', 'tars-mvp.yaml')

def fail_with(message)
  warn(message)
  exit(1)
end

def load_document(path)
  YAML.load_file(path)
rescue StandardError => e
  fail_with("failed to parse #{path}: #{e.message}")
end

def each_node(node, &block)
  yield node
  case node
  when Hash
    node.each_value { |value| each_node(value, &block) }
  when Array
    node.each { |value| each_node(value, &block) }
  end
end

def resolve_ref(document, ref)
  fail_with("only local refs are supported, got #{ref}") unless ref.start_with?('#/')

  ref.delete_prefix('#/').split('/').reduce(document) do |memo, key|
    fail_with("broken ref #{ref}") unless memo.is_a?(Hash) && memo.key?(key)

    memo[key]
  end
end

document = load_document(OPENAPI_PATH)
fail_with('openapi version is required') unless document['openapi']
fail_with('paths section is required') unless document['paths'].is_a?(Hash)

operation_ids = Set.new
refs = []

document.fetch('paths', {}).each do |path, item|
  next unless item.is_a?(Hash)

  item.each do |method, operation|
    next unless %w[get post put patch delete].include?(method)
    next unless operation.is_a?(Hash)

    operation_id = operation['operationId']
    fail_with("missing operationId for #{method.upcase} #{path}") if operation_id.to_s.strip.empty?
    fail_with("duplicate operationId #{operation_id}") if operation_ids.include?(operation_id)

    operation_ids << operation_id
  end
end

each_node(document) do |node|
  next unless node.is_a?(Hash)
  refs << node['$ref'] if node['$ref']
end

refs.each { |ref| resolve_ref(document, ref) }

puts "openapi validation passed: #{operation_ids.size} operations, #{refs.size} refs"
