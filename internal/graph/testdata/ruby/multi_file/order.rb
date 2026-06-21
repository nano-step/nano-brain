class Order < ApplicationRecord
  def calculate_total
    items.sum(&:price)
  end
end
