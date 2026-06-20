class ImportService
  def process
    while has_more?
      batch = fetch_batch
      import_batch(batch)
    end
  end
end
