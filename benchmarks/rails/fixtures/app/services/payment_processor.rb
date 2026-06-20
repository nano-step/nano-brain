class PaymentProcessor
  def process(amount)
    Payment.create(amount: amount, status: "pending")
  end

  def refund(payment_id)
    payment = Payment.find(payment_id)
    payment.update(status: "refunded")
    NotificationService.notify("payment_refunded", payment)
    payment
  end
end
